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

package pd

import (
	"context"
	"time"

	"github.com/pingcap/log"
	"go.etcd.io/etcd/clientv3"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/keepalive"

	"github.com/pingcap-incubator/tidb-dashboard/pkg/config"
)

const (
	TiDBServerInformationPath = "/tidb/server/info"
)

func newZapEncoder(zapcore.EncoderConfig) (zapcore.Encoder, error) {
	logCfg := log.Config{
		DisableTimestamp:    false,
		DisableErrorVerbose: false,
	}
	return log.NewTextEncoder(&logCfg), nil
}

func init() {
	_ = zap.RegisterEncoder("etcd-client", newZapEncoder)
}

func NewEtcdClient(lc fx.Lifecycle, config *config.Config) (*clientv3.Client, error) {
	// TODO: refactor 重构
	// Because etcd client does not support setting logger directly,
	// the configuration of pingcap/log is copied here.
	// 由于etcd客户端不直接支持设置记录器，因此将pingcap / log的配置复制到此处。
	zapCfg := zap.NewProductionConfig()
	zapCfg.Encoding = "etcd-client"
	zapCfg.OutputPaths = []string{"stderr"}
	zapCfg.ErrorOutputPaths = []string{"stderr"}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:        []string{config.PDEndPoint},
		AutoSyncInterval: 30 * time.Second,
		DialTimeout:      5 * time.Second,
		DialOptions: []grpc.DialOption{
			grpc.WithConnectParams(grpc.ConnectParams{
				Backoff: backoff.Config{
					BaseDelay:  1.0 * time.Second,
					Multiplier: 1.6,
					Jitter:     0.2,
					MaxDelay:   3 * time.Second,
				},
				MinConnectTimeout: 20 * time.Second,
			}),
			grpc.WithKeepaliveParams(keepalive.ClientParameters{
				Time:                10 * time.Second,
				Timeout:             3 * time.Second,
				PermitWithoutStream: true,
			}),
		},
		TLS:       config.ClusterTLSConfig,
		LogConfig: &zapCfg,
	})

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			return cli.Close()
		},
	})

	return cli, err
}
