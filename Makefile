.PHONY: swagger_spec yarn_dependencies ui server run dev lint publish_ui_packages

DASHBOARD_PKG := github.com/pingcap-incubator/tidb-dashboard

BUILD_TAGS ?=

SKIP_YARN_INSTALL ?=

LDFLAGS ?=

ifeq ($(SWAGGER),1)
	BUILD_TAGS += swagger_server
endif

ifeq ($(UI),1)
	BUILD_TAGS += ui_server
endif

LDFLAGS += -X "$(DASHBOARD_PKG)/pkg/utils.ReleaseVersion=$(shell git describe --tags --dirty)"
LDFLAGS += -X "$(DASHBOARD_PKG)/pkg/utils.BuildTS=$(shell date -u '+%Y-%m-%d %I:%M:%S')"
LDFLAGS += -X "$(DASHBOARD_PKG)/pkg/utils.GitHash=$(shell git rev-parse HEAD)"
LDFLAGS += -X "$(DASHBOARD_PKG)/pkg/utils.GitBranch=$(shell git rev-parse --abbrev-ref HEAD)"


default:
    SWAGGER=1 make server

lint:
	scripts/lint.sh

dev: lint default

# convert api in Golang code to swagger configuration file
swagger_spec:
	scripts/generate_swagger_spec.sh

yarn_dependencies:
	cd ui
	yarn install --frozen-lockfile

ui: swagger_spec yarn_dependencies
	cd ui &&\
	REACT_APP_DASHBOARD_API_URL="" yarn build

publish_ui_packages: swagger_spec yarn_dependencies
	cd ui &&\
	yarn run build:packages &&\
	yarn run publish:packages

server:
ifeq ($(SWAGGER),1)
	make swagger_spec
endif
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


run_f_d:
	cd bin &&\
	./tidb-dashboard --debug --keyviz-file-start 1574992800 --keyviz-file-end 1575064800

run_f_a:
	cd bin &&\
	./tidb-dashboard --debug --keyviz-file-start 1574992800 --keyviz-file-end 1575064800 --matrix-strategy-mode 1

run_f_mb:
	cd bin &&\
	./tidb-dashboard --debug --keyviz-file-start 1574992800 --keyviz-file-end 1575064800 --matrix-strategy-mode 2

run_f_mg:
	cd bin &&\
	./tidb-dashboard --debug --keyviz-file-start 1574992800 --keyviz-file-end 1575064800 --matrix-strategy-mode 3

all:
	make ui &&\
	make all_server