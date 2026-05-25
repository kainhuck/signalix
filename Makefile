APP := signalix
CMD := ./cmd/signalixd
BIN_DIR := ./bin
BIN := $(BIN_DIR)/$(APP)

.PHONY: help run build test clean tidy fmt proto

help:
	@printf "Available targets:\n"
	@printf "  make run    - run the app\n"
	@printf "  make build  - build the app\n"
	@printf "  make test   - run tests\n"
	@printf "  make fmt    - format Go code\n"
	@printf "  make tidy   - tidy Go modules\n"
	@printf "  make proto  - buf generate (api/proto → api/gen/go)\n"
	@printf "  make clean  - remove build artifacts\n"

run:
	go run $(CMD)

build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN) $(CMD)

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
