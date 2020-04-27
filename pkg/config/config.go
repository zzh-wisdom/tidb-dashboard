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

package config

import (
	"crypto/tls"
	"time"
)

type Config struct {
	DataDir      string
	PDEndPoint   string
	DataInterval time.Duration
	MaxDataDelay time.Duration

	StatInputMode       int
	HeatmapStrategyMode int

	// for test
	StatTest  int
	RegionNum int64

	// TLS config for mTLS authentication between TiDB components.
	ClusterTLSConfig *tls.Config

	// TLS config for mTLS authentication between TiDB and MySQL client.
	TiDBTLSConfig *tls.Config
}
