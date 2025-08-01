# LLMProxy-Go Makefile
# 项目信息
PROJECT_NAME := llmproxyd
MAIN_FILE := cmd/llmproxy/main.go
VERSION := 0.2.1
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# 构建参数
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT) -s -w"
BUILD_FLAGS := -trimpath -tags=jsoniter

# 输出目录
BIN_DIR := bin

# ZIP 文件定义
ZIP_PREFIX := $(PROJECT_NAME)-v$(VERSION)
WINDOWS_AMD64_ZIP := $(BIN_DIR)/$(ZIP_PREFIX)-windows-amd64.zip
LINUX_AMD64_ZIP := $(BIN_DIR)/$(ZIP_PREFIX)-linux-amd64.zip
LINUX_ARM64_ZIP := $(BIN_DIR)/$(ZIP_PREFIX)-linux-arm64.zip
DARWIN_AMD64_ZIP := $(BIN_DIR)/$(ZIP_PREFIX)-darwin-amd64.zip
DARWIN_ARM64_ZIP := $(BIN_DIR)/$(ZIP_PREFIX)-darwin-arm64.zip

# 平台和架构定义
# Windows x64
WINDOWS_AMD64_DIR := $(BIN_DIR)/windows-amd64
WINDOWS_AMD64_BIN := $(WINDOWS_AMD64_DIR)/$(PROJECT_NAME).exe

# Linux x64 和 arm64
LINUX_AMD64_DIR := $(BIN_DIR)/linux-amd64
LINUX_AMD64_BIN := $(LINUX_AMD64_DIR)/$(PROJECT_NAME)
LINUX_ARM64_DIR := $(BIN_DIR)/linux-arm64
LINUX_ARM64_BIN := $(LINUX_ARM64_DIR)/$(PROJECT_NAME)

# macOS x64 和 arm64
DARWIN_AMD64_DIR := $(BIN_DIR)/darwin-amd64
DARWIN_AMD64_BIN := $(DARWIN_AMD64_DIR)/$(PROJECT_NAME)
DARWIN_ARM64_DIR := $(BIN_DIR)/darwin-arm64
DARWIN_ARM64_BIN := $(DARWIN_ARM64_DIR)/$(PROJECT_NAME)

.PHONY: all clean build-all build-windows build-linux build-darwin zip-all zip-windows zip-linux zip-darwin help

# 默认目标：构建所有平台
all: zip-all

# 构建所有平台的二进制文件
build-all: build-windows build-linux build-darwin
	@echo "All builds completed successfully!"

# 构建 Windows x64
build-windows: $(WINDOWS_AMD64_BIN)

$(WINDOWS_AMD64_BIN): $(MAIN_FILE)
	@echo "Building Windows AMD64..."
	@mkdir -p $(WINDOWS_AMD64_DIR)
	@GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build $(BUILD_FLAGS) $(LDFLAGS) -o $@ $(MAIN_FILE)
	@echo "✓ Windows AMD64 build completed: $@"

# 构建 Linux x64 和 arm64
build-linux: $(LINUX_AMD64_BIN) $(LINUX_ARM64_BIN)

$(LINUX_AMD64_BIN): $(MAIN_FILE)
	@echo "Building Linux AMD64..."
	@mkdir -p $(LINUX_AMD64_DIR)
	@GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(BUILD_FLAGS) $(LDFLAGS) -o $@ $(MAIN_FILE)
	@echo "✓ Linux AMD64 build completed: $@"

$(LINUX_ARM64_BIN): $(MAIN_FILE)
	@echo "Building Linux ARM64..."
	@mkdir -p $(LINUX_ARM64_DIR)
	@GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(BUILD_FLAGS) $(LDFLAGS) -o $@ $(MAIN_FILE)
	@echo "✓ Linux ARM64 build completed: $@"

# 构建 macOS x64 和 arm64
build-darwin: $(DARWIN_AMD64_BIN) $(DARWIN_ARM64_BIN)

$(DARWIN_AMD64_BIN): $(MAIN_FILE)
	@echo "Building macOS AMD64..."
	@mkdir -p $(DARWIN_AMD64_DIR)
	@GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build $(BUILD_FLAGS) $(LDFLAGS) -o $@ $(MAIN_FILE)
	@echo "✓ macOS AMD64 build completed: $@"

$(DARWIN_ARM64_BIN): $(MAIN_FILE)
	@echo "Building macOS ARM64..."
	@mkdir -p $(DARWIN_ARM64_DIR)
	@GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build $(BUILD_FLAGS) $(LDFLAGS) -o $@ $(MAIN_FILE)
	@echo "✓ macOS ARM64 build completed: $@"

# 打包所有平台
zip-all: zip-windows zip-linux zip-darwin
	@echo "All zip packages created successfully!"

# 打包 Windows
zip-windows: $(WINDOWS_AMD64_ZIP)

$(WINDOWS_AMD64_ZIP): $(WINDOWS_AMD64_BIN)
	@echo "Creating Windows AMD64 zip package..."
	@cd $(WINDOWS_AMD64_DIR) && zip -9 -j ../$(notdir $@) $(PROJECT_NAME).exe
	@echo "✓ Windows AMD64 zip package created: $@"

# 打包 Linux
zip-linux: $(LINUX_AMD64_ZIP) $(LINUX_ARM64_ZIP)

$(LINUX_AMD64_ZIP): $(LINUX_AMD64_BIN)
	@echo "Creating Linux AMD64 zip package..."
	@cd $(LINUX_AMD64_DIR) && zip -9 -j ../$(notdir $@) $(PROJECT_NAME)
	@echo "✓ Linux AMD64 zip package created: $@"

$(LINUX_ARM64_ZIP): $(LINUX_ARM64_BIN)
	@echo "Creating Linux ARM64 zip package..."
	@cd $(LINUX_ARM64_DIR) && zip -9 -j ../$(notdir $@) $(PROJECT_NAME)
	@echo "✓ Linux ARM64 zip package created: $@"

# 打包 macOS
zip-darwin: $(DARWIN_AMD64_ZIP) $(DARWIN_ARM64_ZIP)

$(DARWIN_AMD64_ZIP): $(DARWIN_AMD64_BIN)
	@echo "Creating macOS AMD64 zip package..."
	@cd $(DARWIN_AMD64_DIR) && zip -9 -j ../$(notdir $@) $(PROJECT_NAME)
	@echo "✓ macOS AMD64 zip package created: $@"

$(DARWIN_ARM64_ZIP): $(DARWIN_ARM64_BIN)
	@echo "Creating macOS ARM64 zip package..."
	@cd $(DARWIN_ARM64_DIR) && zip -9 -j ../$(notdir $@) $(PROJECT_NAME)
	@echo "✓ macOS ARM64 zip package created: $@"

# 清理构建文件
clean:
	@echo "Cleaning build files..."
	@rm -rf $(BIN_DIR)
	@echo "✓ Clean completed"

# 显示构建信息
info:
	@echo "Project: $(PROJECT_NAME)"
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Main File: $(MAIN_FILE)"
	@echo "Output Directory: $(BIN_DIR)"

# 帮助信息
help:
	@echo "LLMProxy-Go Build System"
	@echo ""
	@echo "Available targets:"
	@echo "  all          - Build all platforms (default)"
	@echo "  build-all    - Build all platforms"
	@echo "  build-windows - Build Windows x64 only"
	@echo "  build-linux  - Build Linux x64 and ARM64"
	@echo "  build-darwin - Build macOS x64 and ARM64"
	@echo "  zip-all      - Create zip packages for all platforms"
	@echo "  zip-windows  - Create zip package for Windows x64"
	@echo "  zip-linux    - Create zip packages for Linux x64 and ARM64"
	@echo "  zip-darwin   - Create zip packages for macOS x64 and ARM64"
	@echo "  clean        - Remove all build files"
	@echo "  info         - Show build information"
	@echo "  help         - Show this help message"
	@echo ""
	@echo "Build targets:"
	@echo "  Windows AMD64: $(WINDOWS_AMD64_BIN)"
	@echo "  Linux AMD64:   $(LINUX_AMD64_BIN)"
	@echo "  Linux ARM64:   $(LINUX_ARM64_BIN)"
	@echo "  macOS AMD64:   $(DARWIN_AMD64_BIN)"
	@echo "  macOS ARM64:   $(DARWIN_ARM64_BIN)"
	@echo ""
	@echo "Zip packages:"
	@echo "  Windows AMD64: $(WINDOWS_AMD64_ZIP)"
	@echo "  Linux AMD64:   $(LINUX_AMD64_ZIP)"
	@echo "  Linux ARM64:   $(LINUX_ARM64_ZIP)"
	@echo "  macOS AMD64:   $(DARWIN_AMD64_ZIP)"
	@echo "  macOS ARM64:   $(DARWIN_ARM64_ZIP)"