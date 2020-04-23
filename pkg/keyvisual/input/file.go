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

package input

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pingcap/log"
	"go.uber.org/zap"

	"github.com/pingcap-incubator/tidb-dashboard/pkg/keyvisual/storage"
)

type fileInput struct {
	Path      string
	StartTime time.Time
	EndTime   time.Time
	Now       time.Time
}

// FileInput reads files in the specified time range from the ./data directory.
func FileInput(path string, startTime, endTime time.Time) StatInput {
	return &fileInput{
		Path:      path,
		StartTime: startTime,
		EndTime:   endTime,
		Now:       time.Now(),
	}
}

func (input *fileInput) GetStartTime() time.Time {
	return input.Now.Add(input.StartTime.Sub(input.EndTime))
}

func (input *fileInput) Background(ctx context.Context, stat *storage.Stat) {
	log.Info("keyvisual load files from", zap.Time("start-time", input.StartTime))
	fileTime := input.StartTime
	for !fileTime.After(input.EndTime) {
		regions, err := readFile(input.Path, fileTime)
		fileTime = fileTime.Add(stat.GetDataInterval())
		if err == nil {
			stat.Append(regions, input.Now.Add(fileTime.Sub(input.EndTime)))
		}
	}
	log.Info("keyvisual load files to", zap.Time("end-time", input.EndTime))
}

func readFile(path string, fileTime time.Time) (*RegionsInfo, error) {
	fileName := fileTime.Format("20060102-15-04-05.json")
	filePath := fmt.Sprintf("%s/%s", path, fileName)
	//log.Debug("",zap.String("filename", filePath))
	file, err := os.Open(filePath)
	if err == nil {
		return read(file)
	}
	return nil, err
}
