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

package apiserver

import (
	"net/http"
	"sync"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"

	"github.com/pingcap-incubator/tidb-dashboard/pkg/apiserver/foo"
	"github.com/pingcap-incubator/tidb-dashboard/pkg/apiserver/info"
	"github.com/pingcap-incubator/tidb-dashboard/pkg/apiserver/keyvisual"
	"github.com/pingcap-incubator/tidb-dashboard/pkg/config"
	"github.com/pingcap-incubator/tidb-dashboard/pkg/dbstore"
)

var once sync.Once

func Handler(apiPrefix string, config *config.Config, db *dbstore.DB) http.Handler {
	once.Do(func() {
		// These global modification will be effective only for the first invoke.
		// 这里使用了gin框架
		gin.SetMode(gin.ReleaseMode)
	})

	// 这里使用gin框架生成http.handler,需学习具体用法
	r := gin.New()
	r.Use(cors.Default())
	r.Use(gin.Recovery())
	r.Use(gzip.Gzip(gzip.BestSpeed))
	endpoint := r.Group(apiPrefix)

	//注册一个打招呼的响应？！
	foo.NewService(config).Register(endpoint)
	// 注册一个获取信息的响应
	info.NewService(config, db).Register(endpoint)
	// 注册keyvisual服务
	keyvisual.NewService(config, db).Register(endpoint)

	return r
}
