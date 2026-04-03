APP_NAME := rook-agent
BUILD_DIR := build
PACKAGE_DIR := $(BUILD_DIR)/packages
MAIN_PACKAGE := ./cmd/rook-agent
VERSION ?= 0.0.0-dev

.PHONY: help build test fmt run tidy clean package

help:
	@printf "Available targets:\n"
	@printf "  make build  - Build the %s binary into %s/\n" "$(APP_NAME)" "$(BUILD_DIR)"
	@printf "  make test   - Run Go tests\n"
	@printf "  make fmt    - Format Go sources\n"
	@printf "  make run    - Run the bootstrap executable\n"
	@printf "  make package - Build a Debian package into %s/\n" "$(PACKAGE_DIR)"
	@printf "  make tidy   - Tidy Go module dependencies\n"
	@printf "  make clean  - Remove build artifacts\n"

build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_PACKAGE)

test:
	go test ./...

fmt:
	gofmt -w $$(find cmd internal -name '*.go' -type f | sort)

run:
	go run $(MAIN_PACKAGE)

package: build
	@mkdir -p $(PACKAGE_DIR)
	VERSION=$(VERSION) go run github.com/goreleaser/nfpm/cmd/nfpm@latest pkg -f packaging/nfpm.yaml -p deb -t $(PACKAGE_DIR)/

tidy:
	go mod tidy

clean:
	rm -rf $(BUILD_DIR)
