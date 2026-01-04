# Project definitions
VERSION     := $(shell git describe --always --dirty --long --tags 2>/dev/null)
BUILD_TIME  := $(shell date +%FT%T%z)
LDFLAGS     := -ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"
BIN_FILE    := engine

# Terminal colors
NO_COLOR    := \033[0m
RED_COLOR   := \033[31m
GREEN_COLOR := \033[32m
YELLOW_COLOR:= \033[33m

.PHONY: build clean test bench tidy gen help

build: # Build go files and output binary
	@echo "$(YELLOW_COLOR)======> Build Start$(NO_COLOR)"
	go build -tags=jsoniter $(LDFLAGS) -o $(BIN_FILE) engine.go

clean: # Remove binary and temporary files
	@echo "$(YELLOW_COLOR)======> Clean Start$(NO_COLOR)"
	go clean
	rm -f $(BIN_FILE)

test: # Run all tests
	@echo "$(YELLOW_COLOR)======> Test Start$(NO_COLOR)"
	go test ./... -count=1

bench: # Run benchmarks
	@echo "$(YELLOW_COLOR)======> Benchmark Start$(NO_COLOR)"
	go test -bench .

tidy: # Run go module tidy
	@echo "$(YELLOW_COLOR)======> Go tidy Start$(NO_COLOR)"
	go mod tidy

gen: # Generate code
	@echo "$(YELLOW_COLOR)======> Gen code Start$(NO_COLOR)"
	go generate ./...

help: # Show help
	@echo ""
	@echo "Usage:"
	@echo "  $(YELLOW_COLOR)make$(NO_COLOR) $(GREEN_COLOR)<target>$(NO_COLOR)"
	@echo ""
	@echo "Targets:"
	@awk 'BEGIN {FS = ":.*?# "} /^[a-zA-Z_-]+:.*?#/ {printf "  $(YELLOW_COLOR)%-10s$(GREEN_COLOR)%s$(NO_COLOR)\n", $$1, $$2}' $(MAKEFILE_LIST)
