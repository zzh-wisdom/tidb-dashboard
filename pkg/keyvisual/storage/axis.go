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

	"github.com/pingcap-incubator/tidb-dashboard/pkg/keyvisual/region"

	"github.com/pingcap-incubator/tidb-dashboard/pkg/keyvisual/matrix"
)

// Axis stores consecutive buckets. Each bucket has StartKey, EndKey, and some statistics. The EndKey of each bucket is
// the StartKey of its next bucket. The actual data structure is stored in columns. Therefore satisfies:
// len(Keys) == len(ValuesList[i]) + 1. In particular, ValuesList[0] is the base column.
type MemAxis struct {
	KeysList   [][]string
	ValuesList [][]uint64
}

// CreateAxis checks the given parameters and uses them to build the Axis.
func CreateMemAxis(keysList [][]string, valuesList [][]uint64) MemAxis {
	if len(keysList) == 0 {
		panic("KeysList length must be greater than 0")
	}
	if len(valuesList) == 0 {
		panic("ValuesList length must be greater than 0")
	}
	if len(keysList) != len(valuesList) {
		panic("KeysList length must be equal to ValuesList length")
	}
	for i := range valuesList {
		if len(keysList[i]) <= 1 {
			panic("Keys length must be greater than 1")
		}
		if len(keysList[i]) != len(valuesList[i])+1 {
			panic("Keys length must be equal to Values length + 1")
		}
	}
	return MemAxis{
		KeysList:   keysList,
		ValuesList: valuesList,
	}
}

// CreateEmptyAxis constructs a minimal empty Axis with the given parameters.
func CreateEmptyMemAxis(startKey, endKey string, valuesListLen int) MemAxis {
	keys := []string{startKey, endKey}
	values := []uint64{0}
	keysList := make([][]string, valuesListLen)
	valuesList := make([][]uint64, valuesListLen)
	for i := range valuesList {
		keysList[i] = keys
		valuesList[i] = values
	}
	return CreateMemAxis(keysList, valuesList)
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
func (axis *MemAxis) Range(startKey string, endKey string, tag region.StatTag) matrix.Axis {
	index := int(tag)
	start, end, ok := matrix.KeysRange(axis.KeysList[index], startKey, endKey)
	if !ok {
		return matrix.CreateEmptyAxis(startKey, endKey)
	}
	keys := axis.KeysList[index][start:end]
	values := axis.ValuesList[index][start : end-1]
	return matrix.CreateAxis(keys, values)
}

// Focus uses the base column as the chunk for the Focus operation to obtain the partitioning scheme, and uses this to
// reduce other columns.
//func (axis *MemAxis) Focus(strategy matrix.Strategy, threshold uint64, ratio int, target int) MemAxis {
//	keysList := make([][]string, len(axis.ValuesList))
//	valuesList := make([][]uint64, len(axis.ValuesList))
//	for i := range axis.ValuesList {
//		if target >= len(axis.KeysList[i])-1 {
//			keysList[i] = axis.KeysList[i]
//			valuesList[i] = axis.ValuesList[i]
//		} else {
//			matrixAxis := matrix.CreateAxis(axis.KeysList[i], axis.ValuesList[i])
//			newMatrixAxis := matrixAxis.Focus(strategy, threshold, ratio, target)
//			keysList[i] = newMatrixAxis.Keys
//			valuesList[i] = newMatrixAxis.Values
//		}
//	}
//	return CreateMemAxis(keysList, valuesList)
//}

// Divide uses the base column as the chunk for the Divide operation to obtain the partitioning scheme, and uses this to
// reduce other columns.
func (axis *MemAxis) Divide(strategy matrix.Strategy, target int) MemAxis {
	keysList := make([][]string, len(axis.ValuesList))
	valuesList := make([][]uint64, len(axis.ValuesList))
	for i := range axis.ValuesList {
		if target >= len(axis.KeysList[i])-1 {
			keysList[i] = axis.KeysList[i]
			valuesList[i] = axis.ValuesList[i]
		} else {
			matrixAxis := matrix.CreateAxis(axis.KeysList[i], axis.ValuesList[i])
			newMatrixAxis := matrixAxis.Divide(strategy, target)
			keysList[i] = newMatrixAxis.Keys
			valuesList[i] = newMatrixAxis.Values
		}
	}
	return CreateMemAxis(keysList, valuesList)
}

func Compact(times []time.Time, memAxes []MemAxis, strategy matrix.Strategy) MemAxis {
	if len(memAxes) == 0 {
		return MemAxis{}
	}
	valuesListLen := len(memAxes[0].ValuesList)
	axes := make([]matrix.Axis, len(memAxes))
	keysList := make([][]string, valuesListLen)
	valuesList := make([][]uint64, valuesListLen)
	for i := 0; i < valuesListLen; i++ {
		for j, axis := range memAxes {
			axes[j] = matrix.CreateAxis(axis.KeysList[i], axis.ValuesList[i])
		}
		plane := matrix.CreatePlane(times, axes)
		compactAxis, _ := plane.Compact(strategy)
		keysList[i] = compactAxis.Keys
		valuesList[i] = compactAxis.Values
	}
	return CreateMemAxis(keysList, valuesList)
}