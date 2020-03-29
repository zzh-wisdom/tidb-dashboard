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

package matrix

// Axis stores consecutive buckets. Each bucket has StartKey, EndKey, and some statistics. The EndKey of each bucket is
// the StartKey of its next bucket. The actual data structure is stored in columns. Therefore satisfies:
// len(Keys) == len(ValuesList[i]) + 1. In particular, ValuesList[0] is the base column.
type Axis struct {
	Keys       []string   `json:"keys,omitempty"`
	ValuesList [][]uint64 `json:"values_list,omitempty"`
}

// CreateAxis checks the given parameters and uses them to build the Axis.
func CreateAxis(keys []string, valuesList [][]uint64) Axis {
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
	return Axis{
		Keys:       keys,
		ValuesList: valuesList,
	}
}

// CreateEmptyAxis constructs a minimal empty Axis with the given parameters.
func CreateEmptyAxis(startKey, endKey string, valuesListLen int) Axis {
	keys := []string{startKey, endKey}
	values := []uint64{0}
	valuesList := make([][]uint64, valuesListLen)
	for i := range valuesList {
		valuesList[i] = values
	}
	return CreateAxis(keys, valuesList)
}

// Shrink reduces all statistical values.
func (axis *Axis) Shrink(ratio uint64) {
	for _, values := range axis.ValuesList {
		for i := range values {
			values[i] /= ratio
		}
	}
}

// Range returns a sub Axis with specified range.
func (axis *Axis) Range(startKey string, endKey string) Axis {
	start, end, ok := KeysRange(axis.Keys, startKey, endKey)
	if !ok {
		return CreateEmptyAxis(startKey, endKey, len(axis.ValuesList))
	}
	keys := axis.Keys[start:end]
	valuesList := make([][]uint64, len(axis.ValuesList))
	for i := range valuesList {
		valuesList[i] = axis.ValuesList[i][start : end-1]
	}
	return CreateAxis(keys, valuesList)
}

// Focus uses the base column as the chunk for the Focus operation to obtain the partitioning scheme, and uses this to
// reduce other columns.
func (axis *Axis) Focus(strategy Strategy, threshold uint64, ratio int, target int, chunkStrategy ChunkStrategy) Axis {
	if target >= len(axis.Keys)-1 {
		return *axis
	}

	baseChunk := createChunk(axis.Keys, axis.ValuesList[0], chunkStrategy)
	newChunk := baseChunk.Focus(strategy, threshold, ratio, target)
	valuesListLen := len(axis.ValuesList)
	newValuesList := make([][]uint64, valuesListLen)
	newValuesList[0] = newChunk.Values
	for i := 1; i < valuesListLen; i++ {
		baseChunk.SetValues(axis.ValuesList[i])
		newValuesList[i] = baseChunk.Reduce(newChunk.Keys).Values
	}
	return CreateAxis(newChunk.Keys, newValuesList)
}

// Divide uses the base column as the chunk for the Divide operation to obtain the partitioning scheme, and uses this to
// reduce other columns.
func (axis *Axis) Divide(strategy Strategy, target int, chunkStrategy ChunkStrategy) Axis {
	if target >= len(axis.Keys)-1 {
		return *axis
	}

	baseChunk := createChunk(axis.Keys, axis.ValuesList[0], chunkStrategy)
	newChunk := baseChunk.Divide(strategy, target)
	valuesListLen := len(axis.ValuesList)
	newValuesList := make([][]uint64, valuesListLen)
	newValuesList[0] = newChunk.Values
	for i := 1; i < valuesListLen; i++ {
		baseChunk.SetValues(axis.ValuesList[i])
		newValuesList[i] = baseChunk.Reduce(newChunk.Keys).Values
	}
	return CreateAxis(newChunk.Keys, newValuesList)
}
