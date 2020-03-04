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
	DataDir    string
	PDEndPoint string
	// 安全传输层协议（TLS）用于在两个通信应用程序之间提供保密性和数据完整性。
	TLSConfig  *tls.Config
}
