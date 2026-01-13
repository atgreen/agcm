.PHONY: build install clean test lint fmt release

BINARY_NAME=agcm
BUILD_DIR=./build
RELEASE_DIR=./release
VERSION?=$(shell git describe --tags --always --dirty=+dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -buildvcs=false $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/agcm

install:
	@echo "Installing $(BINARY_NAME)..."
	go install $(LDFLAGS) ./cmd/agcm

clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR) $(RELEASE_DIR)
	@go clean

test:
	@echo "Running tests..."
	go test -v ./...

lint:
	@echo "Linting..."
	golangci-lint run

fmt:
	@echo "Formatting..."
	go fmt ./...

# Development helpers
run:
	go run ./cmd/agcm

run-tui:
	go run ./cmd/agcm

# Build for multiple platforms
build-all: build-linux build-darwin build-windows

build-linux:
	@echo "Building for Linux..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/agcm
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/agcm

build-darwin:
	@echo "Building for macOS..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/agcm
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/agcm

build-windows:
	@echo "Building for Windows..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/agcm

# Release builds with archives and checksums
release: clean
	@echo "Building release $(VERSION)..."
	@mkdir -p $(RELEASE_DIR)
	# Linux amd64
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(RELEASE_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/agcm
	tar -czf $(RELEASE_DIR)/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz -C $(RELEASE_DIR) $(BINARY_NAME)-linux-amd64
	@rm $(RELEASE_DIR)/$(BINARY_NAME)-linux-amd64
	# Linux arm64
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(RELEASE_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/agcm
	tar -czf $(RELEASE_DIR)/$(BINARY_NAME)-$(VERSION)-linux-arm64.tar.gz -C $(RELEASE_DIR) $(BINARY_NAME)-linux-arm64
	@rm $(RELEASE_DIR)/$(BINARY_NAME)-linux-arm64
	# macOS amd64
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(RELEASE_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/agcm
	tar -czf $(RELEASE_DIR)/$(BINARY_NAME)-$(VERSION)-darwin-amd64.tar.gz -C $(RELEASE_DIR) $(BINARY_NAME)-darwin-amd64
	@rm $(RELEASE_DIR)/$(BINARY_NAME)-darwin-amd64
	# macOS arm64
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(RELEASE_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/agcm
	tar -czf $(RELEASE_DIR)/$(BINARY_NAME)-$(VERSION)-darwin-arm64.tar.gz -C $(RELEASE_DIR) $(BINARY_NAME)-darwin-arm64
	@rm $(RELEASE_DIR)/$(BINARY_NAME)-darwin-arm64
	# Windows amd64
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(RELEASE_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/agcm
	cd $(RELEASE_DIR) && zip $(BINARY_NAME)-$(VERSION)-windows-amd64.zip $(BINARY_NAME)-windows-amd64.exe
	@rm $(RELEASE_DIR)/$(BINARY_NAME)-windows-amd64.exe
	# Generate checksums
	cd $(RELEASE_DIR) && sha256sum *.tar.gz *.zip > checksums.txt
	@echo "Release artifacts created in $(RELEASE_DIR)/"
