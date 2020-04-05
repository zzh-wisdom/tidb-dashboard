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
func CreateStorageAxisFromRegions(regions region.RegionsInfo, strategy matrix.Strategy) Axis {
	regionsLen := regions.Len()
	if regionsLen <= 0 {
		panic("At least one RegionInfo")
	}

	keys := regions.GetKeys()
	valuesList := make([][]uint64, len(region.ResponseTags))
	for i, tag := range region.ResponseTags {
		valuesList[i] = regions.GetValues(tag)
	}

	preAxis := CreateStorageAxis(keys, valuesList)
	wash(&preAxis)

	axis := ConciseStorageAxis(preAxis, strategy)
	log.Debug("New StorageAxis", zap.Int("region length", regionsLen), zap.Int("focus keys length", len(axis.Keys)))
	return axis
}

// ConciseStorageAxis divide storageAxis remove storageAxis.ValuesList[0] whose tag is Integration
func ConciseStorageAxis(storageAxis Axis, strategy matrix.Strategy) Axis {
	richStorageAxis := RichStorageAxis(storageAxis)
	axis := richStorageAxis.Divide(strategy, preTarget)
	var ValuesList [][]uint64
	ValuesList = append(ValuesList, axis.ValuesList[1:]...)
	return CreateStorageAxis(axis.Keys, ValuesList)
}

// RichStorageAxis add integration values at storageAxis.ValuesList[0]
func RichStorageAxis(storageAxis Axis) Axis {
	valuesList := make([][]uint64, 1, len(region.ResponseTags))
	writtenBytes := storageAxis.ValuesList[0]
	readBytes := storageAxis.ValuesList[1]
	integration := make([]uint64, len(writtenBytes))
	for i := range integration {
		integration[i] = writtenBytes[i] + readBytes[i]
	}
	valuesList[0] = integration
	valuesList = append(valuesList, storageAxis.ValuesList...)
	return CreateStorageAxis(storageAxis.Keys, valuesList)
}

// IntoMatrixAxis converts StorageAxis to Matrix Axis with the given respTag.
func IntoMatrixAxis(storageAxis Axis, respTag region.StatTag) matrix.Axis {
	richStorageAxis := RichStorageAxis(storageAxis)
	for i, tag := range region.ResponseTags {
		if tag == respTag {
			return matrix.CreateAxis(storageAxis.Keys, richStorageAxis.ValuesList[i])
		}
	}
	panic("unreachable")
}

// TODO: Temporary solution, need to trace the source of dirty data.
func wash(axis *Axis) {
	for i, value := range axis.ValuesList[0] {
		if value >= dirtyValue {
			for j := range region.ResponseTags {
				axis.ValuesList[j][i] = 0
			}
		}
	}
}
