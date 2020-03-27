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
		err := DeletePlane(s.Db, s.Num, s.StartTime)
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
		err := DeletePlane(s.Db, s.Num, s.StartTime)
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

	err := InsertPlane(s.Db, s.Num, endTime, axis)
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
	//s.Load()
	return s
}

// Load data from disk the first time service starts
func (s *Stat) Load() {
	log.Debug("keyviz: load data from dbstore")
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.keyMap.RLock()
	defer s.keyMap.RUnlock()

	// establish start `Plane` for each layer
	createStartPlanes := func() {
		for i, layer := range s.layers {
			err := InsertPlane(s.db, uint8(i), layer.StartTime, matrix.Axis{})
			log.Debug("Insert startTime plane", zap.Uint8("Num", uint8(i)), zap.Time("StartTime", layer.StartTime), zap.Error(err))
		}
	}

	// table planes preprocess
	isExist, err := CreateTablePlaneIfNotExists(s.db)
	if err != nil {
		log.Panic("Create table plane fail", zap.Error(err))
	}
	if !isExist {
		createStartPlanes()
		return
	}

	// load data from db
	num := 0
	for ; ; num++ {
		planes, err := FindPlaneOrderByTime(s.db, uint8(num))
		log.Debug("Load planes", zap.Uint8("Num", uint8(num)), zap.Int("Len", len(planes)-1), zap.Error(err))
		if err != nil || len(planes) == 0 {
			break
		}
		if len(planes) > 1 {
			s.layers[num].Empty = false
		} else if num == 0 {
			// no valid data was stored，clear
			err := ClearTablePlane(s.db)
			log.Debug("Clear table plane", zap.Error(err))
			break
		}

		// the first plane is only used to save starttime,
		s.layers[num].StartTime = planes[0].Time
		s.layers[num].Head = 0
		n := len(planes) - 1
		if n > s.layers[num].Len {
			log.Panic("n cannot be longer than layers[num].Len", zap.Int("n", n), zap.Int("layers[num].Len", s.layers[num].Len), zap.Int("num", num))
		}
		s.layers[num].EndTime = planes[n].Time
		s.layers[num].Tail = (s.layers[num].Head + n) % s.layers[num].Len
		for i, plane := range planes[1 : n+1] {
			s.layers[num].RingTimes[i] = plane.Time
			axis, err := plane.UnmarshalAxis()
			if err != nil {
				panic("Unexpected plane unmarshal error!")
			}
			s.keyMap.SaveKeys(axis.Keys)
			s.layers[num].RingAxes[i] = axis
		}
	}
	if num == 0 {
		createStartPlanes()
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
}

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
