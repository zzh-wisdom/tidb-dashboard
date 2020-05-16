module github.com/pingcap-incubator/tidb-dashboard

go 1.13

require (
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751
	github.com/appleboy/gin-jwt/v2 v2.6.3
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/elazarl/go-bindata-assetfs v1.0.0
	github.com/gin-contrib/gzip v0.0.1
	github.com/gin-gonic/gin v1.5.0
	github.com/go-bindata/go-bindata/v3 v3.1.3
	github.com/go-sql-driver/mysql v1.4.1
	github.com/goccy/go-graphviz v0.0.5
	github.com/google/pprof v0.0.0-20200407044318-7d83b28da2e9
	github.com/google/uuid v1.0.0
	github.com/gtank/cryptopasta v0.0.0-20170601214702-1f550f6f2f69
	github.com/hypnoglow/gormzap v0.3.0
	github.com/jinzhu/gorm v1.9.12
	github.com/joho/godotenv v1.3.0
	github.com/joomcode/errorx v1.0.1
	github.com/pingcap/check v0.0.0-20191216031241-8a5a85928f12
	github.com/pingcap/errors v0.11.5-0.20190809092503-95897b64e011
	github.com/pingcap/kvproto v0.0.0-20200509065137-6a4d5c264a8b
	github.com/pingcap/log v0.0.0-20200117041106-d28c14d3b1cd
	github.com/pingcap/sysutil v0.0.0-20200206130906-2bfa6dc40bcd
	github.com/pkg/errors v0.9.1
	github.com/rs/cors v1.7.0
	github.com/spf13/pflag v1.0.1
	github.com/swaggo/http-swagger v0.0.0-20200103000832-0e9263c4b516
	github.com/swaggo/swag v1.6.5
	go.etcd.io/etcd v0.0.0-20191023171146-3cf2f69b5738
	go.uber.org/fx v1.10.0
	go.uber.org/zap v1.13.0
	golang.org/x/tools v0.0.0-20200403190813-44a64ad78b9b // indirect
	google.golang.org/grpc v1.25.1
)

replace github.com/swaggo/swag => github.com/breeswish/swag v1.6.6-0.20200420031958-719cfd8cfce1
