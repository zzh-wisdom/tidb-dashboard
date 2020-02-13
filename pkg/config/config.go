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
)

type Config struct {
	Version    string
	DataDir    string
	PDEndPoint string
	// 安全传输层协议（TLS）用于在两个通信应用程序之间提供保密性和数据完整性。
	TLSConfig  *tls.Config
/*<<<<<<< HEAD

	// core api mode or file mod
	KeyVisualConfig *keyvisualconfig.Config
}

func NewConfig(ctx context.Context, version string) *Config {
	return &Config{
		Ctx:             ctx,
		Version:         version,
		KeyVisualConfig: keyvisualconfig.NewConfig(),
	}
}

var globalConfig atomic.Value

func init() {
	var cfg *Config = nil
	SetGlobalConfig(cfg)
}

func SetGlobalConfig(cfg *Config) {
	globalConfig.Store(cfg)
}

func GetGlobalConfig() *Config {
	return globalConfig.Load().(*Config)
=======*/
//>>>>>>> 01c268ed0406d52dac4f04d2872978f70a1370b6
}
