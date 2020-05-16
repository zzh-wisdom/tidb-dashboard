// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package keyvisual

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joomcode/errorx"
	"github.com/pingcap/log"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/pingcap-incubator/tidb-dashboard/pkg/apiserver/user"
	"github.com/pingcap-incubator/tidb-dashboard/pkg/config"
	"github.com/pingcap-incubator/tidb-dashboard/pkg/dbstore"
	"github.com/pingcap-incubator/tidb-dashboard/pkg/keyvisual/decorator"
	"github.com/pingcap-incubator/tidb-dashboard/pkg/keyvisual/input"
	"github.com/pingcap-incubator/tidb-dashboard/pkg/keyvisual/matrix"
	"github.com/pingcap-incubator/tidb-dashboard/pkg/keyvisual/region"
	"github.com/pingcap-incubator/tidb-dashboard/pkg/keyvisual/storage"
	"github.com/pingcap-incubator/tidb-dashboard/pkg/utils"
)

const (
	heatmapsMaxDisplayY = 1536 //200

	distanceStrategyRatio = 1.0 / math.Phi
	distanceStrategyLevel = 15
	distanceStrategyCount = 50
)

var (
	ErrNS             = errorx.NewNamespace("error.keyvisual")
	ErrServiceStopped = ErrNS.NewType("service_stopped")

	defaultStatConfig = storage.StatConfig{
		LayersConfig: []storage.LayerConfig{
			{Len: 60, Ratio: 2 / 1},                     // step 1 minutes, total 60, 1 hours (sum: 1 hours)
			{Len: 60 / 2 * 7, Ratio: 6 / 2},             // step 2 minutes, total 210, 7 hours (sum: 8 hours)
			{Len: 60 / 6 * 16, Ratio: 30 / 6},           // step 6 minutes, total 160, 16 hours (sum: 1 days)
			{Len: 60 / 30 * 24 * 6, Ratio: 4 * 60 / 30}, // step 30 minutes, total 288, 6 days (sum: 1 weeks)
			{Len: 24 / 4 * 28, Ratio: 0},                // step 4 hours, total 168, 4 weeks (sum: 5 weeks)
		},
		ReportConfig: storage.ReportConfig{
			ReportInterval:    time.Hour,
			ReportTimeRange:   time.Hour,
			ReportMaxDisplayY: 1536,
			MaxReportNum:      10 * 7 * 24, // 10 weeks
		},

		//ReportConfig: storage.ReportConfig{
		//	ReportInterval:    time.Second * 10,
		//	ReportTimeRange:   time.Second * 10,
		//	ReportMaxDisplayY: 1536,
		//	MaxReportNum:      6,
		//},

		//ReportConfig: storage.ReportConfig{
		//	ReportInterval:    time.Minute * 10,
		//	ReportTimeRange:   time.Minute * 10,
		//	ReportMaxDisplayY: 1536,
		//	MaxReportNum:      6,
		//},
	}
)

type Service struct {
	app    *fx.App
	status *utils.ServiceStatus

	ctx    context.Context
	cancel context.CancelFunc

	config     *config.Config
	cfgManager *config.DynamicConfigManager
	provider   *region.PDDataProvider
	httpClient *http.Client
	db         *dbstore.DB

	statInput input.StatInput
	stat      *storage.Stat
	strategy  matrix.Strategy
}

func NewService(
	lc fx.Lifecycle,
	cfg *config.Config,
	cfgManager *config.DynamicConfigManager,
	provider *region.PDDataProvider,
	httpClient *http.Client,
	db *dbstore.DB,
) *Service {
	s := &Service{
		status:     utils.NewServiceStatus(),
		config:     cfg,
		cfgManager: cfgManager,
		provider:   provider,
		httpClient: httpClient,
		db:         db,
	}

	lc.Append(s.managerHook())

	return s
}

func Register(r *gin.RouterGroup, auth *user.AuthService, s *Service) {
	endpoint := r.Group("/keyvisual")
	endpoint.Use(auth.MWAuthRequired())

	endpoint.GET("/config", s.getDynamicConfig)
	endpoint.PUT("/config", s.setDynamicConfig)

	endpoint.Use(s.status.MWHandleStopped(stoppedHandler))
	endpoint.GET("/heatmaps", s.heatmaps)
}

func (s *Service) IsRunning() bool {
	return s.status.IsRunning()
}

// for test
func (s *Service) testAxisAppend() {
	s.statInput = input.NewSimulationDB(2, 3400)
	s.statInput.Background(context.Background(), s.stat)
	max, min := s.stat.GetAxisMaxAndMinKeysCount()
	log.Info("Axis keys length", zap.Int("max", max), zap.Int("min", min))
	simulationInput := s.statInput.(*input.SimulationDB)
	simulationInput.Resize(s.config.RegionNum)
	TestN := 10
	var regionsList = make([]*input.RegionsInfo, TestN)
	var endTimeList = make([]time.Time, TestN)
	for i := 0; i < TestN; i++ {
		regionsList[i], endTimeList[i] = simulationInput.WorkAndGetInfo()
		fmt.Printf("Num: %d, region length:%d\n", i, regionsList[i].Len())
	}
	statTime := time.Now()
	for i := 0; i < TestN; i++ {
		s.stat.Append(regionsList[i], endTimeList[i])
	}
	endTime := time.Now()
	dur := endTime.Sub(statTime)
	fmt.Printf("[Test] Append axis num: %d, region length: %v, spent time %v.\n", TestN, s.config.RegionNum, dur)
	os.Exit(0)
}

func (s *Service) testStartTime() {
	s.statInput = input.NewSimulationDB(2, 3400)
	s.statInput.Background(context.Background(), s.stat)
	if s.config.IsReportPersist {
		s.stat.FillReport()
	} else {
		s.stat.ClearReport()
	}

	_ = s.Stop(context.Background())
	s.config.StatTest = int(storage.NoTest)
	s.config.StatInputMode = int(input.PeriodicInputMode)
	s.provider.InputMode = int(input.PeriodicInputMode)
	s.config.HeatmapStrategyMode = int(matrix.DistanceHeatmapStrategy)

	statTime := time.Now()
	_ = s.Start(context.Background())
	endTime := time.Now()
	dur := endTime.Sub(statTime)
	fmt.Printf("[Test] Service start, Full of storage data, spent time %v.\n", dur)
	os.Exit(0)
}

func (s *Service) testGenerateHeatmap() {
	s.statInput = input.NewSimulationDB(2, 3400)
	s.statInput.Background(context.Background(), s.stat)

	var mtx matrix.Matrix
	statTime := time.Now()
	TestN := 10
	for i := 0; i < TestN; i++ {
		mtx = s.stat.GenerateMatrix()
		fmt.Printf("[Test] heatmap %d, X: %d, Y: %d.\n", i, len(mtx.TimeAxis), len(mtx.Keys))
	}
	endTime := time.Now()
	dur := endTime.Sub(statTime)
	fmt.Printf("[Test] Generate heatmap num: %d, X: %d, Y: %d, spent time %v.\n", TestN, len(mtx.TimeAxis), len(mtx.Keys), dur)
	os.Exit(0)
}

func (s *Service) Start(ctx context.Context) error {
	if s.IsRunning() {
		return nil
	}

	s.ctx, s.cancel = context.WithCancel(ctx)

	if s.config.StatTest != int(storage.NoTest) && s.config.StatInputMode != int(input.SimulationMode) {
		panic("error parameters")
	}
	if s.config.StatTest == int(storage.NoTest) && !s.config.IsKeyIntern {
		log.Warn("error parameters, IsKeyIntern should be true")
	}
	defaultStatConfig.DataInterval = s.config.DataInterval
	defaultStatConfig.MaxDelayTime = s.config.MaxDataDelay
	defaultStatConfig.StatTest = storage.StatTest(s.config.StatTest)
	defaultStatConfig.IsKeyIntern = s.config.IsKeyIntern
	newStatFunc := newStat
	if s.config.StatTest != int(storage.NoTest) && s.config.StatInputMode == int(input.SimulationMode) {
		newStatFunc = newStatForTest
	}

	s.app = fx.New(
		fx.Logger(utils.NewFxPrinter()),
		fx.Provide(
			newWaitGroup,
			newStrategy,
			newStatFunc,
			s.provideLocals,
			input.NewStatInput,
			decorator.TiDBLabelStrategy,
		),
		fx.Populate(&s.statInput, &s.stat, &s.strategy),
		fx.Invoke(
			// Must be at the end
			s.status.Register,
		),
	)

	if err := s.app.Err(); err != nil {
		return err
	}
	if err := s.app.Start(s.ctx); err != nil {
		return err
	}

	statTest := storage.StatTest(s.config.StatTest)
	if statTest != storage.NoTest && s.config.StatInputMode == int(input.SimulationMode) {
		switch statTest {
		case storage.BenchmarkAxisAppend:
			s.testAxisAppend()
		case storage.BenchmarkRestore:
			s.testStartTime()
		case storage.BenchmarkGenerateHeatmap:
			s.testGenerateHeatmap()
		}
	}

	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	if !s.IsRunning() {
		return nil
	}

	s.cancel()
	err := s.app.Stop(ctx)

	// drop
	s.app = nil
	s.stat = nil
	s.strategy = nil
	s.ctx = nil
	s.cancel = nil

	return err
}

// @Summary Key Visual Heatmaps
// @Description Heatmaps in a given range to visualize TiKV usage
// @Produce json
// @Param startkey query string false "The start of the key range"
// @Param endkey query string false "The end of the key range"
// @Param starttime query int false "The start of the time range (Unix)"
// @Param endtime query int false "The end of the time range (Unix)"
// @Param type query string false "Main types of data" Enums(written_bytes, read_bytes, written_keys, read_keys, integration)
// @Param way query string false "The way of heatmap generation" Enums(matrix, report)
// @Success 200 {object} matrix.Matrix
// @Router /keyvisual/heatmaps [get]
// @Security JwtAuth
// @Failure 401 {object} utils.APIError "Unauthorized failure"
func (s *Service) heatmaps(c *gin.Context) {
	startKey := c.Query("startkey")
	endKey := c.Query("endkey")
	startTimeString := c.Query("starttime")
	endTimeString := c.Query("endtime")
	typ := c.Query("type")
	way := c.Query("way")

	endTime := time.Now()
	startTime := endTime.Add(-360 * time.Minute)
	if startTimeString != "" {
		tsSec, err := strconv.ParseInt(startTimeString, 10, 64)
		if err != nil {
			log.Error("parse ts failed", zap.Error(err))
			c.JSON(http.StatusBadRequest, "bad request")
			return
		}
		startTime = time.Unix(tsSec, 0)
	}
	if endTimeString != "" {
		tsSec, err := strconv.ParseInt(endTimeString, 10, 64)
		if err != nil {
			log.Error("parse ts failed", zap.Error(err))
			c.JSON(http.StatusBadRequest, "bad request")
			return
		}
		endTime = time.Unix(tsSec, 0)
	}
	if !(startTime.Before(endTime) && (endKey == "" || startKey < endKey)) {
		c.JSON(http.StatusBadRequest, "bad request")
		return
	}

	log.Debug("Request matrix",
		zap.Time("start-time", startTime),
		zap.Time("end-time", endTime),
		zap.String("start-key", startKey),
		zap.String("end-key", endKey),
		zap.String("type", typ),
	)

	if startKeyBytes, err := hex.DecodeString(startKey); err == nil {
		startKey = string(startKeyBytes)
	} else {
		c.JSON(http.StatusBadRequest, "bad request")
		return
	}
	if endKeyBytes, err := hex.DecodeString(endKey); err == nil {
		endKey = string(endKeyBytes)
	} else {
		c.JSON(http.StatusBadRequest, "bad request")
		return
	}

	baseTag := region.IntoTag(typ)

	// TODO: for report test, which needs to be deleted later.
	//if endKey != "" || startKey != "" {
	//	resp, isFind := s.stat.GetReport(startTime, endTime, startKey, endKey, baseTag)
	//	if isFind {
	//		c.JSON(http.StatusOK, resp)
	//		return
	//	}
	//}

	if way == "report" {
		resp, isFind := s.stat.GetReport(startTime, endTime, startKey, endKey, baseTag)
		if isFind {
			c.JSON(http.StatusOK, resp)
		}
		// TODO: deal with the case when finding no report
	} else {
		plane := s.stat.Range(startTime, endTime, startKey, endKey, baseTag)
		resp := plane.Pixel(s.strategy, heatmapsMaxDisplayY, baseTag.String())
		resp.RangeKey(startKey, endKey)
		// TODO: An expedient to reduce data transmission, which needs to be deleted later.
		//resp.DataMap = map[string][][]uint64{
		//	typ: resp.DataMap[typ],
		//}
		// ----------
		c.JSON(http.StatusOK, resp)
	}
}

func (s *Service) provideLocals() (*config.Config, *region.PDDataProvider, *http.Client, *dbstore.DB) {
	return s.config, s.provider, s.httpClient, s.db
}

func newWaitGroup(lc fx.Lifecycle) *sync.WaitGroup {
	wg := &sync.WaitGroup{}
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			wg.Wait()
			return nil
		},
	})
	return wg
}

func newStrategy(lc fx.Lifecycle, wg *sync.WaitGroup, cfg *config.Config, labelStrategy decorator.LabelStrategy) matrix.Strategy {
	strategyConfig := matrix.NewStrategyConfig(matrix.HeatmapStrategy(cfg.HeatmapStrategyMode), distanceStrategyRatio, distanceStrategyLevel, distanceStrategyCount)
	log.Info("HeatmapStrategy", zap.String("Mode", matrix.HeatmapStrategy(cfg.HeatmapStrategyMode).String()))
	return matrix.NewStrategy(lc, wg, strategyConfig, labelStrategy)
}

func newStat(lc fx.Lifecycle, wg *sync.WaitGroup, cfg *config.Config, in input.StatInput, strategy matrix.Strategy, db *dbstore.DB) *storage.Stat {
	statInputMode := input.StatInputMode(cfg.StatInputMode)
	log.Debug("stat input mode", zap.String("Mode", statInputMode.String()))
	isPeriodicInputMode := statInputMode == input.PeriodicInputMode
	stat := storage.NewStat(lc, defaultStatConfig, strategy, in.GetStartTime(), isPeriodicInputMode, db)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			wg.Add(1)
			go func() {
				in.Background(ctx, stat)
				wg.Done()
			}()
			return nil
		},
	})

	return stat
}

func newStatForTest(lc fx.Lifecycle, wg *sync.WaitGroup, cfg *config.Config, in input.StatInput, strategy matrix.Strategy, db *dbstore.DB) *storage.Stat {
	statInputMode := input.StatInputMode(cfg.StatInputMode)
	log.Debug("stat input mode", zap.String("Mode", statInputMode.String()))
	isPeriodicInputMode := statInputMode == input.PeriodicInputMode
	stat := storage.NewStat(lc, defaultStatConfig, strategy, in.GetStartTime(), isPeriodicInputMode, db)
	return stat
}

func stoppedHandler(c *gin.Context) {
	_ = c.AbortWithError(http.StatusNotFound, ErrServiceStopped.NewWithNoMessage())
}
