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

// Package input defines several different data inputs.
package input

import (
	"context"
	"time"

	"github.com/pingcap-incubator/tidb-dashboard/pkg/keyvisual/region"
	"github.com/pingcap-incubator/tidb-dashboard/pkg/keyvisual/storage"
)

// StatInput is the interface that different data inputs need to implement.
type StatInput interface {
	GetStartTime() time.Time
	Background(ctx context.Context, stat *storage.Stat)
}

func NewStatInput(provider *region.PDDataProvider) StatInput {
	inputMode := StatInputMode(provider.InputMode)
	switch inputMode {
	case PeriodicInputMode:
		return PeriodicInput(provider.PeriodicGetter)
	case FileInputMode:
		startTime := time.Unix(provider.FileStartTime, 0)
		endTime := time.Unix(provider.FileEndTime, 0)
		return FileInput(provider.FilePath, startTime, endTime)
	case SimulationMode:
		return NewSimulationDB(TableNum, InitRegionNum)
	default:
		panic("unreachable")
	}
}

type StatInputMode int

const (
	PeriodicInputMode StatInputMode = 0
	FileInputMode     StatInputMode = 1
	SimulationMode    StatInputMode = 2
)

func (s StatInputMode) String() string {
	switch s {
	case PeriodicInputMode:
		return "PeriodicInputMode"
	case FileInputMode:
		return "FileInputMode"
	case SimulationMode:
		return "SimulationMode"
	default:
		panic("unreachable")
	}
}
