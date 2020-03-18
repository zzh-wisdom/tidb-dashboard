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
	"fmt"
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

	Db *dbstore.DB
	// Hierarchical mechanism
	Strategy matrix.Strategy
	Ratio    int
	Next     *layerStat
}

func newLayerStat(num uint8, conf LayerConfig, strategy matrix.Strategy, startTime time.Time, db *dbstore.DB) *layerStat {
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
		Db:        db,
		Strategy:  strategy,
		Ratio:     conf.Ratio,
		Next:      nil,
	}
}

// Reduce merges ratio axes and append to next layerStat
func (s *layerStat) Reduce() {
	if s.Ratio == 0 || s.Next == nil {
		// 有问题
		//plane := NewPlane(s.StartTime, s.Num, nil)
		//err := s.Db.Where(plane).Delete(&Plane{}).Error
		err := s.Db.Where("layer_num = ? AND time = ?", s.Num, s.StartTime).Delete(&Plane{}).Error
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
		// 有问题
		//plane := NewPlane(s.StartTime, s.Num, nil)
		//err := s.Db.Where(plane).Delete(&Plane{}).Error
		err := s.Db.Where("layer_num = ? AND time = ?", s.Num, s.StartTime).Delete(&Plane{}).Error
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

	plane, _ := NewPlaneMode(endTime, s.Num, axis, Mode)
	err := s.Db.Create(plane).Error
	log.Debug("Insert Plane", zap.Uint8("Num", s.Num), zap.Int("Location", s.Tail), zap.Time("Time", endTime), zap.Error( err))

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
}

// Stat is composed of multiple layerStats.
type Stat struct {
	mutex  sync.RWMutex
	layers []*layerStat

	keyMap   matrix.KeyMap
	strategy matrix.Strategy

	provider *region.PDDataProvider

	db *dbstore.DB
}

// NewStat generates a Stat based on the configuration.
func NewStat(lc fx.Lifecycle, wg *sync.WaitGroup, provider *region.PDDataProvider, cfg StatConfig, strategy matrix.Strategy, startTime time.Time, db *dbstore.DB) *Stat {
	layers := make([]*layerStat, len(cfg.LayersConfig))
	for i, c := range cfg.LayersConfig {
		layers[i] = newLayerStat(uint8(i), c, strategy, startTime, db)
		if i > 0 {
			layers[i-1].Next = layers[i]
		}
	}
	s := &Stat{
		layers:   layers,
		strategy: strategy,
		provider: provider,
		db:       db,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			wg.Add(1)
			go func() {
				s.rebuildRegularly(ctx)
				wg.Done()
			}()
			return nil
		},
	})

	s.Load()

	return s
}

func stringsAreSorted(strings []string) bool {
	if len(strings) <= 1 {
		return true
	}
	high := len(strings)
	if strings[high-1] == "" {
		high--
	}
	for i := 0; i < high-1; i++ {
		if strings[i] > strings[i+1] {
			return false
		}
	}
	return true
}

// Load data from disk the first time service starts
func (s *Stat) Load() {
	log.Debug("Keyviz: load data from dbstore")
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.keyMap.RLock()
	defer s.keyMap.RUnlock()

	// 存储各层的起始时间轴
	createStartPlanes := func() {
		for i, layer := range s.layers {
			plane := NewPlane(layer.StartTime, uint8(i), nil)
			err := s.db.Create(plane).Error
			log.Debug("Insert startTime planes", zap.Uint8("Num", uint8(i)), zap.Time("Num", layer.StartTime), zap.Error(err))
		}
	}

	//s.db.DropTable("planes")
	// Preprocessing
	if !checkTable(s.db, tableName) {
		err := s.db.CreateTable(&Plane{}).Error
		if err != nil {
			log.Panic("Create table plane", zap.Error(err))
		}
		createStartPlanes()
		return
	}

	// load data from db
	num := 0
	var planes []Plane

	//debug
	var count int
	err := s.db.Order("layer_num").Find(&planes).Count(&count).Error
	fmt.Println("Plane count:", count, err)

	for ; ; num++ {
		err := s.db.Where("layer_num = ?", num).Order("Time").Find(&planes).Error
		//fmt.Println(planes.)
		fmt.Println("num:", num, "len(planes)", len(planes)-1)
		if err != nil || len(planes) == 0 {
			break
		}
		if len(planes) == 1 {
			s.layers[num].Empty = true
			if num == 0 {
				// 第一层也只有用于存储开始时间的空plane，说明上次启动没有存储到数据,清空db中的无用数据
				err = s.db.Delete(&Plane{}).Error
				log.Debug("Clear table plane", zap.Error(err))
				break
			}
		} else {
			s.layers[num].Empty = false
		}
		// 第一个数据Plane用于保存该层的起始时间
		s.layers[num].StartTime = planes[0].Time
		s.layers[num].Head = 0
		n := len(planes) - 1
		if n > s.layers[num].Len {
			n = s.layers[num].Len
			log.Warn("len(plane) higher than Len", zap.Int("len(plane)", len(planes)-1), zap.Int("Len", s.layers[num].Len))
			err := s.db.Where("layer_num = ? AND time > ?", num, planes[n].Time).Delete(&Plane{}).Error
			log.Debug("Delete warning planes", zap.Error(err))
		}
		s.layers[num].EndTime = planes[n].Time
		s.layers[num].Tail = (s.layers[num].Head + n) % s.layers[num].Len

		for i, plane := range planes[1:n+1] {
			s.layers[num].RingTimes[i] = plane.Time
			axis, err := plane.UnmarshalMode(Mode)
			if err != nil {
				panic("Unexpected error!")
			}
			//isSorted := stringsAreSorted(axis.Keys)
			//fmt.Println("isSorted:", isSorted)
			s.keyMap.SaveKeys(axis.Keys)
			s.layers[num].RingAxes[i] = axis
			//fmt.Println("load key len:", len(axis.Keys), "type num:", len(axis.ValuesList), "statices num:", len(axis.ValuesList[0]))
			//log.Debug("Load plane", zap.Uint8("Num", uint8(num)), zap.Time("S", plane.Time))
		}
	}

	if num == 0 {
		createStartPlanes()
	}
	for _, layer := range s.layers {
		fmt.Println("Num:", layer.Num)
		fmt.Println("StartTime:", layer.StartTime, "EndTime:", layer.EndTime)
		fmt.Println("Head:", layer.Head, "Tail:", layer.Tail, "Len:", layer.Len, "Empty:", layer.Empty)
		fmt.Println("len(RingAxes):", len(layer.RingAxes))
	}
}

func (s *Stat) rebuildKeyMap() {
	s.keyMap.Lock()
	defer s.keyMap.Unlock()
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.keyMap.Map = sync.Map{}

	for _, layer := range s.layers {
		for _, axis := range layer.RingAxes {
			if len(axis.Keys) > 0 {
				s.keyMap.SaveKeys(axis.Keys)
			}
		}
	}
}

func (s *Stat) rebuildRegularly(ctx context.Context) {
	ticker := time.NewTicker(time.Hour * 24)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.rebuildKeyMap()
		}
	}
}

// Append adds the latest full statistics.
func (s *Stat) Append(regions region.RegionsInfo, endTime time.Time) {
	if regions.Len() == 0 {
		return
	}
	axis := CreateStorageAxis(regions, s.strategy)

	s.keyMap.RLock()
	defer s.keyMap.RUnlock()
	s.keyMap.SaveKeys(axis.Keys)

	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.layers[0].Append(axis, endTime)

	//compare(s)
}

//func compare(stat *Stat)  {
//	config := []LayerConfig{
//		{Len: 60, Ratio: 2 / 1},                     // step 1 minutes, total 60, 1 hours (sum: 1 hours)
//		{Len: 60 / 2 * 7, Ratio: 6 / 2},             // step 2 minutes, total 210, 7 hours (sum: 8 hours)
//		{Len: 60 / 6 * 16, Ratio: 30 / 6},           // step 6 minutes, total 160, 16 hours (sum: 1 days)
//		{Len: 60 / 30 * 24 * 6, Ratio: 4 * 60 / 30}, // step 30 minutes, total 288, 6 days (sum: 1 weeks)
//		{Len: 24 / 4 * 28, Ratio: 0},                // step 4 hours, total 168, 4 weeks (sum: 5 weeks)
//	}
//	layers := make([]*layerStat, len(config))
//	for i, c := range config {
//		layers[i] = newLayerStat(uint8(i), c, stat.strategy, stat.layers[0].StartTime, stat.db)
//		if i > 0 {
//			layers[i-1].Next = layers[i]
//		}
//	}
//	newStat := &Stat{
//		layers:   layers,
//		strategy: stat.strategy,
//		provider: stat.provider,
//		db: stat.db,
//	}
//	newStat.Load()
//	fmt.Println(reflect.DeepEqual(stat.layers,newStat.layers))
//
//	fmt.Println("layer len:", len(stat.layers) == len(newStat.layers))
//	for i := range stat.layers {
//		fmt.Println("layer(",i,")StartTime:", stat.layers[i].StartTime.String() == newStat.layers[i].StartTime.String())
//		fmt.Println("layer(",i,")EndTime:", stat.layers[i].EndTime.String() == newStat.layers[i].EndTime.String())
//		fmt.Println("layer(",i,")Empty:", stat.layers[i].Empty == newStat.layers[i].Empty)
//		fmt.Println("layer(",i,")Head:", stat.layers[i].Head == newStat.layers[i].Head)
//		fmt.Println("layer(",i,")Tail:", stat.layers[i].Tail == newStat.layers[i].Tail)
//		fmt.Println("layer(",i,")Len:", stat.layers[i].Len == newStat.layers[i].Len)
//		fmt.Println("layer(",i,")Num:", stat.layers[i].Num == newStat.layers[i].Num)
//		fmt.Println("layer(",i,")len(RingTimes):", len(stat.layers[i].RingTimes) == len(newStat.layers[i].RingTimes))
//		fmt.Println("layer(",i,")len(RingAxes):", len(stat.layers[i].RingAxes) == len(newStat.layers[i].RingAxes))
//	}
//
//	for i:=stat.layers[0].Head; i<stat.layers[0].Tail; i++ {
//		for j := range stat.layers[0].RingAxes[i].Keys {
//			if stat.layers[0].RingAxes[i].Keys[j] != newStat.layers[0].RingAxes[i].Keys[j] {
//				fmt.Println(i,j)
//				fmt.Println(stat.layers[0].RingAxes[i].Keys[j])
//				fmt.Println(newStat.layers[0].RingAxes[i].Keys[j])
//			}
//		}
//	}
//}

func (s *Stat) rangeRoot(startTime, endTime time.Time) ([]time.Time, []matrix.Axis) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.layers[0].Range(startTime, endTime)
}

// Range returns a sub Plane with specified range.
func (s *Stat) Range(startTime, endTime time.Time, startKey, endKey string, baseTag region.StatTag) matrix.Plane {
	s.keyMap.RLock()
	defer s.keyMap.RUnlock()
	s.keyMap.SaveKey(&startKey)
	s.keyMap.SaveKey(&endKey)

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
