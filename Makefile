APP_NAME := rook-agent
BUILD_DIR := build
MAIN_PACKAGE := ./cmd/rook-agent

.PHONY: help build test fmt run tidy clean

help:
	@printf "Available targets:\n"
	@printf "  make build  - Build the %s binary into %s/\n" "$(APP_NAME)" "$(BUILD_DIR)"
	@printf "  make test   - Run Go tests\n"
	@printf "  make fmt    - Format Go sources\n"
	@printf "  make run    - Run the bootstrap executable\n"
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

tidy:
	go mod tidy

clean:
	rm -rf $(BUILD_DIR)
