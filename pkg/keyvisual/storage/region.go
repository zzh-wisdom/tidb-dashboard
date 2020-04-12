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

package storage

import (
	"github.com/pingcap/log"
	"go.uber.org/zap"

	"github.com/pingcap-incubator/tidb-dashboard/pkg/keyvisual/matrix"
	"github.com/pingcap-incubator/tidb-dashboard/pkg/keyvisual/region"
)

// Source data pre processing parameters.
const (
	// preThreshold   = 128
	// preRatioTarget = 512
	preTarget = 3072

	dirtyValue = 1 << 30
)

// CreateStorageAxis converts the RegionsInfo to a StorageAxis.
func CreateStorageAxisFromRegions(keyMap *matrix.KeyMap, regions region.RegionsInfo, strategy matrix.Strategy) MemAxis {
	regionsLen := regions.Len()
	if regionsLen <= 0 {
		panic("At least one RegionInfo")
	}

	keys := regions.GetKeys()
	keyMap.RLock()
	keyMap.SaveKeys(keys)
	keyMap.RUnlock()

	keysList := make([][]string, len(region.Tags))
	valuesList := make([][]uint64, len(region.Tags))
	for i, tag := range region.Tags {
		keysList[i] = keys
		valuesList[i] = regions.GetValues(tag)
	}

	preAxis := CreateMemAxis(keysList, valuesList)
	wash(&preAxis)
	newAxis := preAxis.Divide(strategy, preTarget)

	lengths := make([]int, len(newAxis.KeysList))
	for i, keys := range newAxis.KeysList {
		lengths[i] = len(keys)
	}
	log.Debug("New MemAxis", zap.Int("region length", regionsLen), zap.Ints("focus keys length", lengths))
	return newAxis
}

// TODO: Temporary solution, need to trace the source of dirty data.
func wash(axis *MemAxis) {
	for i, value := range axis.ValuesList[0] {
		if value >= dirtyValue {
			for j := range region.Tags {
				axis.ValuesList[j][i] = 0
			}
		}
	}
}
