.PHONY: build install uninstall clean tidy test test-coverage dev run daemon release release-dry-run desktop desktop-dev

BINARY := ugudu
BUILD_DIR := bin
DIST_DIR := dist
INSTALL_PATH := /usr/local/bin
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION)"

# Platforms for cross-compilation
PLATFORMS := darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64

# Desktop app directory
DESKTOP_DIR := desktop

# Build the binary
build:
	@echo "Building $(BINARY)..."
	@go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/ugudu
	@echo "Built $(BUILD_DIR)/$(BINARY)"

# Build and install to system
install: build
	@echo "Installing $(BINARY) to $(INSTALL_PATH)..."
	@sudo cp $(BUILD_DIR)/$(BINARY) $(INSTALL_PATH)/$(BINARY)
	@sudo chmod +x $(INSTALL_PATH)/$(BINARY)
	@echo "Installed! Run 'ugudu --help' to get started."

# Uninstall from system
uninstall:
	@echo "Removing $(BINARY) from $(INSTALL_PATH)..."
	@sudo rm -f $(INSTALL_PATH)/$(BINARY)
	@echo "Uninstalled."

# Development: build and install (quick iteration)
dev: install
	@echo "Ready for testing!"

# Clean build artifacts
clean:
	@rm -rf $(BUILD_DIR)
	@echo "Cleaned."

# Tidy go modules
tidy:
	@go mod tidy

# Run all tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run tests for specific package
test-pkg:
	@go test -v ./internal/$(PKG)/...

# Start daemon (for development)
daemon: build
	@./$(BUILD_DIR)/$(BINARY) daemon --tcp :8080

# Quick run
run: build
	@./$(BUILD_DIR)/$(BINARY) $(ARGS)

# Release: build all platform binaries
release:
	@echo "Building release $(VERSION) for all platforms..."
	@rm -rf $(DIST_DIR)
	@mkdir -p $(DIST_DIR)
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*}; \
		GOARCH=$${platform#*/}; \
		OUTPUT=$(BINARY); \
		[ "$$GOOS" = "windows" ] && OUTPUT=$(BINARY).exe; \
		DIR=$(DIST_DIR)/$(BINARY)_$(VERSION)_$${GOOS}_$${GOARCH}; \
		mkdir -p $$DIR; \
		echo "Building $$GOOS/$$GOARCH..."; \
		GOOS=$$GOOS GOARCH=$$GOARCH go build $(LDFLAGS) -o $$DIR/$$OUTPUT ./cmd/ugudu; \
		cp README.md $$DIR/; \
		[ -f LICENSE ] && cp LICENSE $$DIR/; \
	done
	@echo "Creating archives..."
	@cd $(DIST_DIR) && for dir in $(BINARY)_$(VERSION)_darwin_* $(BINARY)_$(VERSION)_linux_*; do \
		tar -czvf $${dir}.tar.gz $$dir && rm -rf $$dir; \
	done
	@cd $(DIST_DIR) && for dir in $(BINARY)_$(VERSION)_windows_*; do \
		zip -r $${dir}.zip $$dir && rm -rf $$dir; \
	done
	@cd $(DIST_DIR) && shasum -a 256 *.tar.gz *.zip > checksums.txt
	@echo ""
	@echo "Release $(VERSION) built successfully!"
	@ls -la $(DIST_DIR)

# Dry run release (show what would be built)
release-dry-run:
	@echo "Would build release $(VERSION) for:"
	@for platform in $(PLATFORMS); do echo "  - $$platform"; done

# Create and push git tag
tag:
	@if [ -z "$(TAG)" ]; then echo "Usage: make tag TAG=v1.0.0"; exit 1; fi
	@echo "Creating tag $(TAG)..."
	@git tag -a $(TAG) -m "Release $(TAG)"
	@git push origin $(TAG)
	@echo "Tag $(TAG) pushed. GitHub Actions will create the release."

# Desktop app development
desktop-dev:
	@echo "Starting desktop app in dev mode..."
	@cd $(DESKTOP_DIR) && cargo tauri dev

# Desktop app build (current platform)
desktop:
	@echo "Building desktop app..."
	@cd $(DESKTOP_DIR) && cargo tauri build
	@echo "Desktop app built in $(DESKTOP_DIR)/src-tauri/target/release/bundle/"

# Full release with desktop
release-full: release desktop
	@echo "Full release complete!"

# Show help
help:
	@echo "Ugudu Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make build          Build the binary to ./bin/ugudu"
	@echo "  make install        Build and install to /usr/local/bin (requires sudo)"
	@echo "  make uninstall      Remove from /usr/local/bin"
	@echo "  make dev            Build and install (for quick iteration)"
	@echo "  make test           Run all tests"
	@echo "  make test-coverage  Run tests with coverage report"
	@echo "  make daemon         Build and start daemon"
	@echo "  make clean          Remove build artifacts"
	@echo "  make tidy           Tidy go modules"
	@echo ""
	@echo "Desktop App:"
	@echo "  make desktop-dev    Run desktop app in development mode"
	@echo "  make desktop        Build desktop app for current platform"
	@echo ""
	@echo "Release:"
	@echo "  make release        Build CLI binaries for all platforms"
	@echo "  make release-full   Build CLI + desktop app"
	@echo "  make release-dry-run Show what would be built"
	@echo "  make tag TAG=v1.0.0 Create and push a git tag"
	@echo ""
	@echo "Distribution:"
	@echo "  npm:        npm install -g @arcslash/ugudu"
	@echo "  Homebrew:   brew install arcslash/tap/ugudu"
	@echo "  Chocolatey: choco install ugudu"
	@echo ""
	@echo "After 'make install', use 'ugudu' directly from anywhere."
