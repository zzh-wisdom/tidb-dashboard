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

import (
	"time"
)

// Plane stores consecutive axes. Each axis has StartTime, EndTime. The EndTime of each axis is the StartTime of its
// next axis. Therefore satisfies:
// len(Times) == len(Axes) + 1
type Plane struct {
	Times []time.Time
	Axes  []Axis
}

// CreatePlane checks the given parameters and uses them to build the Plane.
func CreatePlane(times []time.Time, axes []Axis) Plane {
	if len(times) <= 1 {
		panic("Times length must be greater than 1")
	}
	return Plane{
		Times: times,
		Axes:  axes,
	}
}

// CreateEmptyPlane constructs a minimal empty Plane with the given parameters.
func CreateEmptyPlane(startTime, endTime time.Time, startKey, endKey string) Plane {
	return CreatePlane([]time.Time{startTime, endTime}, []Axis{CreateEmptyAxis(startKey, endKey)})
}

// Compact compacts Plane into an axis.
func (plane *Plane) Compact(strategy Strategy) (Axis, interface{}) {
	// get compact Axis keys
	keySet := make(map[string]struct{})
	unlimitedEnd := false
	for _, axis := range plane.Axes {
		end := len(axis.Keys) - 1
		endKey := axis.Keys[end]
		if endKey == "" {
			unlimitedEnd = true
		} else {
			keySet[endKey] = struct{}{}
		}
		for _, key := range axis.Keys[:end] {
			keySet[key] = struct{}{}
		}
	}

	var compactKeys []string
	if unlimitedEnd {
		compactKeys = MakeKeysWithUnlimitedEnd(keySet)
	} else {
		compactKeys = MakeKeys(keySet)
	}
	compactAxis := CreateZeroAxis(compactKeys)

	helper := strategy.GenerateHelper(plane.Axes, compactAxis.Keys)
	for i, c := range plane.Axes {
		strategy.Split(compactAxis, c, SplitAdd, i, helper)
	}
	return compactAxis, helper
}

// Pixel pixelates Plane into a matrix with a number of rows close to the target.
func (plane *Plane) Pixel(strategy Strategy, target int, displayTag string) Matrix {
	compactAxis, helper := plane.Compact(strategy)
	baseKeys := compactAxis.Divide(strategy, target).Keys
	matrix := CreateMatrix(strategy, plane.Times, baseKeys)

	axesLen := len(plane.Axes)
	data := make([][]uint64, axesLen)
	goCompactAxis := CreateZeroAxis(compactAxis.Keys)
	for i, axis := range plane.Axes {
		goCompactAxis.Clear()
		strategy.Split(goCompactAxis, axis, SplitTo, i, helper)
		data[i] = goCompactAxis.Reduce(strategy, baseKeys).Values
	}
	matrix.DataMap[displayTag] = data
	return matrix
}