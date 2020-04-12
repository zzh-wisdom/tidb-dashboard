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
	"sync"

	"go.uber.org/fx"

	"github.com/pingcap-incubator/tidb-dashboard/pkg/keyvisual/decorator"
)

type SplitTag int

const (
	SplitTo  SplitTag = iota // Direct assignment after split
	SplitAdd                 // Add to original value after split
)

// splitStrategy is an allocation scheme. GenerateHelper is used to generate a helper for a Plane. Split uses this
// helper to split a chunk of columns.
type splitStrategy interface {
	GenerateHelper(axes []Axis, compactKeys []string) interface{}
	Split(dst, src Axis, tag SplitTag, axesIndex int, helper interface{})
}

// Strategy is part of the customizable strategy in Matrix generation.
type Strategy interface {
	splitStrategy
	decorator.LabelStrategy
	GetAxisStrategy() AxisStrategy
}

type StrategyMode int

const (
	DistanceStrategyMode    StrategyMode = 0
	AverageStrategyMode     StrategyMode = 1
	MaxBorderStrategyMode   StrategyMode = 2
	MaxGradientStrategyMode StrategyMode = 3
)

func (sm StrategyMode) String() string {
	switch sm {
	case AverageStrategyMode:
		return "AverageStrategy"
	case DistanceStrategyMode:
		return "DistanceStrategyMode"
	case MaxBorderStrategyMode:
		return "MaxBorderStrategyMode"
	case MaxGradientStrategyMode:
		return "MaxGradientStrategyMode"
	default:
		panic("unreachable")
	}
}

func NewStrategy(lc fx.Lifecycle, wg *sync.WaitGroup, config *StrategyConfig, label decorator.LabelStrategy) Strategy {
	switch config.Mode {
	case AverageStrategyMode:
		return AverageStrategy(label)
	case DistanceStrategyMode:
		return DistanceStrategy(lc, wg, label, config.DistanceStrategyRatio, config.DistanceStrategyLevel, config.distanceStrategyCount)
	case MaxBorderStrategyMode:
		return MaximumStrategy(label, MaxBorderStrategy)
	case MaxGradientStrategyMode:
		return MaximumStrategy(label, MaxGradientStrategy)
	default:
		panic("unreachable")
	}
}