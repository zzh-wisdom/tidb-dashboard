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

package tidb

import (
	"database/sql/driver"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"github.com/pingcap/log"
	"go.uber.org/zap"

	// MySQL driver used by gorm
	_ "github.com/jinzhu/gorm/dialects/mysql"
)

const (
	envTidbOverrideEndpointKey = "TIDB_OVERRIDE_ENDPOINT"
)

func (f *Forwarder) getDBConnProps() (host string, port int, err error) {
	info, err := f.getServerInfo()
	if err == nil {
		host = info.IP
		port = info.Port
	}
	return
}

func (f *Forwarder) OpenTiDB(user string, pass string) (*gorm.DB, error) {
	var addr string
	addr = os.Getenv(envTidbOverrideEndpointKey)
	if len(addr) < 1 {
		host, port, err := f.getDBConnProps()
		if err != nil {
			return nil, err
		}
		addr = fmt.Sprintf("%s:%d", host, port)
	}
	dsnConfig := mysql.NewConfig()
	dsnConfig.Net = "tcp"
	dsnConfig.Addr = addr
	dsnConfig.User = user
	dsnConfig.Passwd = pass
	dsnConfig.Timeout = time.Second
	if f.config.TiDBTLSConfig != nil {
		dsnConfig.TLSConfig = "tidb"
	}
	dsn := dsnConfig.FormatDSN()

	db, err := gorm.Open("mysql", dsn)
	if err != nil {
		if _, ok := err.(*net.OpError); ok || err == driver.ErrBadConn {
			if strings.HasPrefix(addr, "0.0.0.0:") {
				log.Warn("The IP reported by TiDB is 0.0.0.0, which may not have the -advertise-address option")
			}
			return nil, ErrTiDBConnFailed.Wrap(err, "failed to connect to TiDB")
		} else if mysqlErr, ok := err.(*mysql.MySQLError); ok {
			if mysqlErr.Number == 1045 {
				return nil, ErrTiDBAuthFailed.New("bad TiDB username or password")
			}
		}
		log.Warn("unknown error occurred while OpenTiDB", zap.Error(err))
		return nil, err
	}

	return db, nil
}
