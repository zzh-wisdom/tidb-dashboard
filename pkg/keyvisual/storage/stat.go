// Copyright 2019 PingCAP, Inc.
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

// Package storage stores the input axes in order, and can get a Plane by time interval.
package storage

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/pingcap/log"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/pingcap-incubator/tidb-dashboard/pkg/dbstore"
	"github.com/pingcap-incubator/tidb-dashboard/pkg/keyvisual/matrix"
	"github.com/pingcap-incubator/tidb-dashboard/pkg/keyvisual/region"
)

// LayerConfig is the configuration of layerStat.
type LayerConfig struct {
	Len   int
	Ratio int
}

// layerStat is a layer in Stat. It uses a circular queue structure and can store up to Len Axes. Whenever the data is
// full, the Ratio Axes will be compacted into an Axis and added to the next layer.
type layerStat struct {
	StartTime time.Time
	EndTime   time.Time
	RingAxes  []matrix.Axis
	RingTimes []time.Time

	Num   uint8
	Head  int
	Tail  int
	Empty bool
	Len   int

	BUM *BackUpManage
	// Hierarchical mechanism
	Strategy matrix.Strategy
	Ratio    int
	Next     *layerStat
}

func newLayerStat(num uint8, conf LayerConfig, strategy matrix.Strategy, startTime time.Time, bum *BackUpManage) *layerStat {
	return &layerStat{
		StartTime: startTime,
		EndTime:   startTime,
		RingAxes:  make([]matrix.Axis, conf.Len),
		RingTimes: make([]time.Time, conf.Len),
		Num:       num,
		Head:      0,
		Tail:      0,
		Empty:     true,
		Len:       conf.Len,
		BUM:       bum,
		Strategy:  strategy,
		Ratio:     conf.Ratio,
		Next:      nil,
	}
}

// Reduce merges ratio axes and append to next layerStat
func (s *layerStat) Reduce() {
	if s.Ratio == 0 || s.Next == nil {
		err := s.BUM.DeletePlane(s.Num, s.StartTime, s.RingAxes[s.Head])
		log.Debug("Delete Plane", zap.Uint8("Num", s.Num), zap.Int("Location", s.Head), zap.Time("Time", s.StartTime), zap.Error(err))

		s.StartTime = s.RingTimes[s.Head]
		s.RingAxes[s.Head] = matrix.Axis{}
		s.Head = (s.Head + 1) % s.Len
		return
	}

	times := make([]time.Time, 0, s.Ratio+1)
	times = append(times, s.StartTime)
	axes := make([]matrix.Axis, 0, s.Ratio)

	for i := 0; i < s.Ratio; i++ {
		err := s.BUM.DeletePlane(s.Num, s.StartTime, s.RingAxes[s.Head])
		log.Debug("Delete Plane", zap.Uint8("Num", s.Num), zap.Int("Location", s.Head), zap.Time("Time", s.StartTime), zap.Error(err))

		s.StartTime = s.RingTimes[s.Head]
		times = append(times, s.StartTime)
		axes = append(axes, s.RingAxes[s.Head])
		s.RingAxes[s.Head] = matrix.Axis{}
		s.Head = (s.Head + 1) % s.Len
	}

	plane := matrix.CreatePlane(times, axes)
	newAxis := plane.Compact(s.Strategy)
	newAxis = IntoResponseAxis(newAxis, region.Integration)
	newAxis = IntoStorageAxis(newAxis, s.Strategy)
	newAxis.Shrink(uint64(s.Ratio))
	s.Next.Append(newAxis, s.StartTime)
}

// Append appends a key axis to layerStat.
func (s *layerStat) Append(axis matrix.Axis, endTime time.Time) {
	if s.Head == s.Tail && !s.Empty {
		s.Reduce()
	}

	err := s.BUM.InsertPlane(s.Num, endTime, axis)
	log.Debug("Insert Plane", zap.Uint8("Num", s.Num), zap.Int("Location", s.Tail), zap.Time("Time", endTime), zap.Error(err))

	s.RingAxes[s.Tail] = axis
	s.RingTimes[s.Tail] = endTime
	s.Empty = false
	s.EndTime = endTime
	s.Tail = (s.Tail + 1) % s.Len
}

// Range gets the specify plane in the time range.
func (s *layerStat) Range(startTime, endTime time.Time) (times []time.Time, axes []matrix.Axis) {
	if s.Next != nil {
		times, axes = s.Next.Range(startTime, endTime)
	}

	if s.Empty || (!(startTime.Before(s.EndTime) && endTime.After(s.StartTime))) {
		return times, axes
	}

	size := s.Tail - s.Head
	if size <= 0 {
		size += s.Len
	}

	start := sort.Search(size, func(i int) bool {
		return s.RingTimes[(s.Head+i)%s.Len].After(startTime)
	})
	end := sort.Search(size, func(i int) bool {
		return !s.RingTimes[(s.Head+i)%s.Len].Before(endTime)
	})
	if end != size {
		end++
	}

	n := end - start
	start = (s.Head + start) % s.Len

	// add StartTime
	if len(times) == 0 {
		if start == s.Head {
			times = append(times, s.StartTime)
		} else {
			times = append(times, s.RingTimes[(start-1+s.Len)%s.Len])
		}
	}

	if start+n <= s.Len {
		times = append(times, s.RingTimes[start:start+n]...)
		axes = append(axes, s.RingAxes[start:start+n]...)
	} else {
		times = append(times, s.RingTimes[start:s.Len]...)
		times = append(times, s.RingTimes[0:start+n-s.Len]...)
		axes = append(axes, s.RingAxes[start:s.Len]...)
		axes = append(axes, s.RingAxes[0:start+n-s.Len]...)
	}

	return times, axes
}

// StatConfig is the configuration of Stat.
type StatConfig struct {
	LayersConfig []LayerConfig
	ReportConfig
}

// Stat is composed of multiple layerStats.
type Stat struct {
	mutex  sync.RWMutex
	layers []*layerStat

	strategy matrix.Strategy

	provider *region.PDDataProvider

	reportManage *ReportManage
	backUpManage *BackUpManage
}

// NewStat generates a Stat based on the configuration.
func NewStat(lc fx.Lifecycle, provider *region.PDDataProvider, cfg StatConfig, strategy matrix.Strategy, startTime time.Time, db *dbstore.DB) *Stat {
	layers := make([]*layerStat, len(cfg.LayersConfig))
	bum := NewBackUpManage(db)
	for i, c := range cfg.LayersConfig {
		layers[i] = newLayerStat(uint8(i), c, strategy, startTime, bum)
		if i > 0 {
			layers[i-1].Next = layers[i]
		}
	}
	s := &Stat{
		layers:       layers,
		strategy:     strategy,
		provider:     provider,
		reportManage: NewReportManage(db, startTime, cfg.ReportConfig),
		backUpManage: bum,
	}

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			s.Restore(startTime)
			return nil
		},
	})

	return s
}

// Restore data from disk the first time service starts
func (s *Stat) Restore(startTime time.Time) {
	log.Debug("keyviz: restore data from dbstore")
	s.mutex.Lock()

	planes := s.backUpManage.Restore(len(s.layers), startTime)
	if len(planes) != 0 {
		for num, plane := range planes {
			log.Debug("Load planes", zap.Uint8("Num", uint8(num)), zap.Int("Len", len(plane.Axes)-1))

			s.layers[num].Empty = len(plane.Axes) <= 1
			s.layers[num].StartTime = plane.Times[0]
			s.layers[num].Head = 0
			n := len(plane.Axes)
			if n > s.layers[num].Len {
				log.Fatal("n cannot be longer than layers[num].Len", zap.Int("n", n), zap.Int("layers[num].Len", s.layers[num].Len), zap.Int("num", num))
			}
			s.layers[num].EndTime = plane.Times[n]
			s.layers[num].Tail = (s.layers[num].Head + n) % s.layers[num].Len
			tempTime := plane.Times[1 : n+1]
			for i, axis := range plane.Axes {
				s.layers[num].RingTimes[i] = tempTime[i]
				s.layers[num].RingAxes[i] = axis
			}
		}
	}

	s.mutex.Unlock()

	// restore Report data
	initReportTime := s.reportManage.ReportTime
	err := s.reportManage.RestoreReport()
	if err != nil {
		log.Panic("restore report error", zap.Error(err))
	}
	//log.Debug("all reports", zap.Times("EndTime", s.reportManage.ReportEndTimes))
	if s.reportManage.IsNeedReport(initReportTime.Add(-s.reportManage.ReportInterval)) {
		newMatrix := s.generateMatrix()
		err := s.reportManage.InsertReport(newMatrix)
		if err != nil {
			log.Warn("InsertReport error", zap.Error(err))
		}
		s.reportManage.ReportTime = initReportTime
	}
	log.Debug("next report time", zap.Time("ReportTime", s.reportManage.ReportTime))
}

func (s *Stat) GetReport(startTime, endTime time.Time, startKey, endKey string) (report matrix.Matrix, isFind bool) {
	report, isFind, err := s.reportManage.FindReport(endTime)
	if err != nil {
		log.Warn("GetReport error", zap.Error(err))
	}
	report.RangeTimeAndKey(startTime, endTime, startKey, endKey)
	log.Debug("GetReport", zap.Time("EndTime", endTime), zap.Bool("isFind", isFind))
	return
}

// Append adds the latest full statistics.
func (s *Stat) Append(regions region.RegionsInfo, endTime time.Time) {
	if regions.Len() == 0 {
		return
	}
	axis := CreateStorageAxis(regions, s.strategy)

	s.mutex.Lock()
	//defer s.mutex.Unlock()
	s.layers[0].Append(axis, endTime)
	s.mutex.Unlock()

	if !s.reportManage.IsNeedReport(endTime) {
		return
	}
	newMatrix := s.generateMatrix()
	err := s.reportManage.InsertReport(newMatrix)
	if err != nil {
		log.Warn("InsertReport error", zap.Error(err))
	}
	log.Debug("next report time", zap.Time("ReportTime", s.reportManage.ReportTime))
}

func (s *Stat) generateMatrix() matrix.Matrix {
	reportStartTime := s.reportManage.ReportTime.Add(-s.reportManage.ReportInterval)
	reportEndTime := s.reportManage.ReportTime
	log.Debug("new report", zap.Time("StartTime", reportStartTime), zap.Time("EndTime", reportEndTime))
	plane := s.Range(reportStartTime, reportEndTime, "", "", region.Integration)
	newMatrix := plane.Pixel(s.strategy, s.reportManage.ReportMaxDisplayY, region.GetDisplayTags(region.Integration))
	return newMatrix
}

func (s *Stat) rangeRoot(startTime, endTime time.Time) ([]time.Time, []matrix.Axis) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.layers[0].Range(startTime, endTime)
}

// Range returns a sub Plane with specified range.
func (s *Stat) Range(startTime, endTime time.Time, startKey, endKey string, baseTag region.StatTag) matrix.Plane {
	s.backUpManage.InternKey(&startKey)
	s.backUpManage.InternKey(&endKey)

	times, axes := s.rangeRoot(startTime, endTime)

	if len(times) <= 1 {
		return matrix.CreateEmptyPlane(startTime, endTime, startKey, endKey, len(region.ResponseTags))
	}

	for i, axis := range axes {
		axis = axis.Range(startKey, endKey)
		axis = IntoResponseAxis(axis, baseTag)
		axes[i] = axis
	}
	return matrix.CreatePlane(times, axes)
}
