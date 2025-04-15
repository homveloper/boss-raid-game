# Makefile for Text-Based Boss Raid Game

# Application name
APP_NAME := boss-raid-game

# Output directory
OUTPUT_DIR := ./bin

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get

# Build flags
LDFLAGS := -ldflags="-s -w"

# Detect OS
ifeq ($(OS),Windows_NT)
	DELETE_CMD := if exist $(OUTPUT_DIR) rmdir /s /q $(OUTPUT_DIR)
	MKDIR_CMD := if not exist $(OUTPUT_DIR) mkdir $(OUTPUT_DIR)
	PATHSEP := \\
	EXE_EXT := .exe
else
	DELETE_CMD := rm -rf $(OUTPUT_DIR)
	MKDIR_CMD := mkdir -p $(OUTPUT_DIR)
	PATHSEP := /
	EXE_EXT :=
endif

# Default target
.PHONY: all
all: clean build

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning..."
	@$(DELETE_CMD)
	@$(GOCLEAN)
	@echo "Cleaned!"

# Create output directory
$(OUTPUT_DIR):
	@echo "Creating output directory..."
	@$(MKDIR_CMD)

# Build for the current platform
.PHONY: build
build: $(OUTPUT_DIR)
	@echo "Building for current platform..."
	@$(GOBUILD) -o $(OUTPUT_DIR)$(PATHSEP)$(APP_NAME)$(EXE_EXT) $(LDFLAGS) .
	@echo "Build complete: $(OUTPUT_DIR)$(PATHSEP)$(APP_NAME)$(EXE_EXT)"

# Build for all platforms
.PHONY: build-all
build-all: clean windows-builds darwin-builds linux-builds

# Windows builds
.PHONY: windows-builds
windows-builds: $(OUTPUT_DIR)
	@echo "Building for Windows..."
	@GOOS=windows GOARCH=386 $(GOBUILD) -o $(OUTPUT_DIR)$(PATHSEP)$(APP_NAME)_windows_386.exe $(LDFLAGS) .
	@GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(OUTPUT_DIR)$(PATHSEP)$(APP_NAME)_windows_amd64.exe $(LDFLAGS) .
	@GOOS=windows GOARCH=arm GOARM=7 $(GOBUILD) -o $(OUTPUT_DIR)$(PATHSEP)$(APP_NAME)_windows_arm.exe $(LDFLAGS) .
	@GOOS=windows GOARCH=arm64 $(GOBUILD) -o $(OUTPUT_DIR)$(PATHSEP)$(APP_NAME)_windows_arm64.exe $(LDFLAGS) .

# macOS builds
.PHONY: darwin-builds
darwin-builds: $(OUTPUT_DIR)
	@echo "Building for macOS..."
	@GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(OUTPUT_DIR)$(PATHSEP)$(APP_NAME)_darwin_amd64 $(LDFLAGS) .
	@GOOS=darwin GOARCH=arm64 $(GOBUILD) -o $(OUTPUT_DIR)$(PATHSEP)$(APP_NAME)_darwin_arm64 $(LDFLAGS) .

# Linux builds
.PHONY: linux-builds
linux-builds: $(OUTPUT_DIR)
	@echo "Building for Linux..."
	@GOOS=linux GOARCH=386 $(GOBUILD) -o $(OUTPUT_DIR)$(PATHSEP)$(APP_NAME)_linux_386 $(LDFLAGS) .
	@GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(OUTPUT_DIR)$(PATHSEP)$(APP_NAME)_linux_amd64 $(LDFLAGS) .
	@GOOS=linux GOARCH=arm GOARM=7 $(GOBUILD) -o $(OUTPUT_DIR)$(PATHSEP)$(APP_NAME)_linux_arm $(LDFLAGS) .
	@GOOS=linux GOARCH=arm64 $(GOBUILD) -o $(OUTPUT_DIR)$(PATHSEP)$(APP_NAME)_linux_arm64 $(LDFLAGS) .

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	@$(GOTEST) -v ./...

# Run the application
.PHONY: run
run:
	@echo "Running application..."
	@$(GOCMD) run main.go

# Install dependencies
.PHONY: deps
deps:
	@echo "Installing dependencies..."
	@$(GOGET) -v ./...

# Help
.PHONY: help
help:
	@echo "Make targets:"
	@echo "  all        - Clean and build for current platform"
	@echo "  clean      - Remove build artifacts"
	@echo "  build      - Build for current platform"
	@echo "  build-all  - Build for all supported platforms"
	@echo "  test       - Run tests"
	@echo "  run        - Run the application"
	@echo "  deps       - Install dependencies"
	@echo "  help       - Show this help message"
