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

package storage

import (
	"time"

	"github.com/pingcap-incubator/tidb-dashboard/pkg/keyvisual/matrix"
)

// Axis stores consecutive buckets. Each bucket has StartKey, EndKey, and some statistics. The EndKey of each bucket is
// the StartKey of its next bucket. The actual data structure is stored in columns. Therefore satisfies:
// len(Keys) == len(ValuesList[i]) + 1. In particular, ValuesList[0] is the base column.
type MemAxis struct {
	Keys       []string
	ValuesList [][]uint64
}

// CreateAxis checks the given parameters and uses them to build the Axis.
func CreateStorageAxis(keys []string, valuesList [][]uint64) MemAxis {
	keysLen := len(keys)
	if keysLen <= 1 {
		panic("Keys length must be greater than 1")
	}
	if len(valuesList) == 0 {
		panic("ValuesList length must be greater than 0")
	}
	for _, values := range valuesList {
		if keysLen != len(values)+1 {
			panic("Keys length must be equal to Values length + 1")
		}
	}
	return MemAxis{
		Keys:       keys,
		ValuesList: valuesList,
	}
}

// CreateEmptyAxis constructs a minimal empty Axis with the given parameters.
func CreateEmptyStorageAxis(startKey, endKey string, valuesListLen int) MemAxis {
	keys := []string{startKey, endKey}
	values := []uint64{0}
	valuesList := make([][]uint64, valuesListLen)
	for i := range valuesList {
		valuesList[i] = values
	}
	return CreateStorageAxis(keys, valuesList)
}

// Shrink reduces all statistical values.
func (axis *MemAxis) Shrink(ratio uint64) {
	for _, values := range axis.ValuesList {
		for i := range values {
			values[i] /= ratio
		}
	}
}

// Range returns a sub Axis with specified range.
func (axis *MemAxis) Range(startKey string, endKey string) MemAxis {
	start, end, ok := matrix.KeysRange(axis.Keys, startKey, endKey)
	if !ok {
		return CreateEmptyStorageAxis(startKey, endKey, len(axis.ValuesList))
	}
	keys := axis.Keys[start:end]
	valuesList := make([][]uint64, len(axis.ValuesList))
	for i := range valuesList {
		valuesList[i] = axis.ValuesList[i][start : end-1]
	}
	return CreateStorageAxis(keys, valuesList)
}

// Focus uses the base column as the chunk for the Focus operation to obtain the partitioning scheme, and uses this to
// reduce other columns.
func (axis *MemAxis) Focus(strategy matrix.Strategy, threshold uint64, ratio int, target int) MemAxis {
	if target >= len(axis.Keys)-1 {
		return *axis
	}

	baseAxis := matrix.CreateAxis(axis.Keys, axis.ValuesList[0])
	newAxis := baseAxis.Focus(strategy, threshold, ratio, target)
	valuesListLen := len(axis.ValuesList)
	newValuesList := make([][]uint64, valuesListLen)
	newValuesList[0] = newAxis.Values
	for i := 1; i < valuesListLen; i++ {
		baseAxis.SetValues(axis.ValuesList[i])
		newValuesList[i] = baseAxis.Reduce(newAxis.Keys).Values
	}
	return CreateStorageAxis(newAxis.Keys, newValuesList)
}

// Divide uses the base column as the chunk for the Divide operation to obtain the partitioning scheme, and uses this to
// reduce other columns.
func (axis *MemAxis) Divide(strategy matrix.Strategy, target int) MemAxis {
	if target >= len(axis.Keys)-1 {
		return *axis
	}

	baseAxis := matrix.CreateAxis(axis.Keys, axis.ValuesList[0])
	newAxis := baseAxis.Divide(strategy, target)
	valuesListLen := len(axis.ValuesList)
	newValuesList := make([][]uint64, valuesListLen)
	newValuesList[0] = newAxis.Values
	for i := 1; i < valuesListLen; i++ {
		baseAxis.SetValues(axis.ValuesList[i])
		newValuesList[i] = baseAxis.Reduce(newAxis.Keys).Values
	}
	return CreateStorageAxis(newAxis.Keys, newValuesList)
}

func Compact(times []time.Time, StorageAxes []MemAxis, strategy matrix.Strategy) MemAxis {
	if len(StorageAxes) == 0 {
		return MemAxis{}
	}
	axes := make([]matrix.Axis, len(StorageAxes))
	for i, axis := range StorageAxes {
		axes[i] = matrix.CreateAxis(axis.Keys, axis.ValuesList[0])
	}
	plane := matrix.CreatePlane(times, axes)
	compactAxis, helper := plane.Compact(strategy)
	valuesListLen := len(StorageAxes[0].ValuesList)
	valuesList := make([][]uint64, valuesListLen)
	valuesList[0] = compactAxis.Values
	for j := 1; j < valuesListLen; j++ {
		compactAxis.SetZeroValues()
		for i, axis := range StorageAxes {
			axes[i].SetValues(axis.ValuesList[j])
			strategy.Split(compactAxis, axes[i], matrix.SplitAdd, i, helper)
		}
		valuesList[j] = compactAxis.Values
	}
	return CreateStorageAxis(compactAxis.Keys, valuesList)
}