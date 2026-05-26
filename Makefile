APP := signalixd
CMD := ./cmd/signalixd
CLI := signalix
CLI_CMD := ./cmd/signalix
BIN_DIR := ./bin
BIN := $(BIN_DIR)/$(APP)
CLI_BIN := $(BIN_DIR)/$(CLI)
VERSION ?= dev
CLI_LDFLAGS := -X github.com/kainhuck/signalix/internal/cli/cmd.version=$(VERSION)

.PHONY: help run build build-cli install-cli test clean tidy fmt proto

help:
	@printf "Available targets:\n"
	@printf "  make run         - run signalixd\n"
	@printf "  make build       - build signalixd\n"
	@printf "  make build-cli   - build signalix CLI (./bin/signalix)\n"
	@printf "  make install-cli - go install signalix CLI\n"
	@printf "  make test        - run tests\n"
	@printf "  make fmt         - format Go code\n"
	@printf "  make tidy        - tidy Go modules\n"
	@printf "  make proto       - buf generate (api/proto → api/gen/go)\n"
	@printf "  make clean       - remove build artifacts\n"

run:
	go run $(CMD)

build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN) $(CMD)

build-cli:
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(CLI_LDFLAGS)" -o $(CLI_BIN) $(CLI_CMD)

install-cli:
	go install -ldflags "$(CLI_LDFLAGS)" $(CLI_CMD)

test:
	go test ./...

fmt:
	go fmt ./...

tidy:
	go mod tidy

proto:
	cd api/proto && GOPROXY=https://proxy.golang.org,direct GOSUMDB=sum.golang.org go run github.com/bufbuild/buf/cmd/buf@v1.47.2 generate

clean:
	rm -rf $(BIN_DIR)
