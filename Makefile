.PHONY: install_tools lint dev yarn_dependencies ui server run

DASHBOARD_PKG := github.com/pingcap-incubator/tidb-dashboard

BUILD_TAGS ?=

LDFLAGS ?=

ifeq ($(UI),1)
	BUILD_TAGS += ui_server
endif

LDFLAGS += -X "$(DASHBOARD_PKG)/pkg/utils.ReleaseVersion=$(shell git describe --tags --dirty)"
LDFLAGS += -X "$(DASHBOARD_PKG)/pkg/utils.BuildTS=$(shell date -u '+%Y-%m-%d %I:%M:%S')"
LDFLAGS += -X "$(DASHBOARD_PKG)/pkg/utils.GitHash=$(shell git rev-parse HEAD)"
LDFLAGS += -X "$(DASHBOARD_PKG)/pkg/utils.GitBranch=$(shell git rev-parse --abbrev-ref HEAD)"


default: server

install_tools:
	scripts/install_go_tools.sh

lint:
	scripts/lint.sh

dev: lint default

yarn_dependencies: install_tools
	cd ui &&\
	yarn install --frozen-lockfile

ui: yarn_dependencies
	cd ui &&\
	REACT_APP_DASHBOARD_API_URL="" yarn build

server: install_tools
	scripts/generate_swagger_spec.sh
ifeq ($(UI),1)
	scripts/embed_ui_assets.sh
endif
	go build -o bin/tidb-dashboard -ldflags '$(LDFLAGS)' -tags "${BUILD_TAGS}" cmd/tidb-dashboard/main.go

all_server:
	UI=1 SWAGGER=1 make server

run:
	bin/tidb-dashboard --debug

run_p_d:
	bin/tidb-dashboard --debug

run_p_a:
	bin/tidb-dashboard --debug --matrix-strategy-mode 1

run_p_mb:
	bin/tidb-dashboard --debug --matrix-strategy-mode 2

run_p_mg:
	bin/tidb-dashboard --debug --matrix-strategy-mode 3


run_fs_d:
	./bin/tidb-dashboard --debug --keyviz-file-start 1574992800 --keyviz-file-end 1575064800 --keyviz-input-mode 1 --max-data-delay 10m

run_fs_a:
	./bin/tidb-dashboard --debug --keyviz-file-start 1574992800 --keyviz-file-end 1575064800 --keyviz-input-mode 1 --max-data-delay 10m --matrix-strategy-mode 1

run_fs_mb:
	./bin/tidb-dashboard --debug --keyviz-file-start 1574992800 --keyviz-file-end 1575064800 --keyviz-input-mode 1 --max-data-delay 10m --matrix-strategy-mode 2

run_fs_mg:
	./bin/tidb-dashboard --debug --keyviz-file-start 1574992800 --keyviz-file-end 1575064800 --keyviz-input-mode 1 --max-data-delay 10m --matrix-strategy-mode 3

run_s_d:
	./bin/tidb-dashboard --debug --keyviz-input-mode 2

all:
	make ui &&\
	make all_server

getter:
	go build -o bin/get-regionsinfo cmd/get-regionsinfo/main.go

run_getter:
	./bin/get-regionsinfo --dir regions-default --interval 1s

test_axis_append_5k:
	./bin/tidb-dashboard --keyviz-input-mode 2 --stat-test 2 --region-num 5000

test_axis_append_10k:
	./bin/tidb-dashboard --keyviz-input-mode 2 --stat-test 2 --region-num 10000

test_axis_append_50k:
	./bin/tidb-dashboard --keyviz-input-mode 2 --stat-test 2 --region-num 50000

test_axis_append_100k:
	./bin/tidb-dashboard --keyviz-input-mode 2 --stat-test 2 --region-num 100000

test_axis_append_500k:
	./bin/tidb-dashboard --keyviz-input-mode 2 --stat-test 2 --region-num 500000

test_axis_append_1000k:
	./bin/tidb-dashboard --keyviz-input-mode 2 --stat-test 2 --region-num 1000000


test_service_start:
	./bin/tidb-dashboard --keyviz-input-mode 2 --stat-test 1

test_generate_heatmap:
	./bin/tidb-dashboard --keyviz-input-mode 2 --stat-test 3

