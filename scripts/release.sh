#!/bin/bash
set -e

VERSION="${1:-0.1.0}"
DIST_DIR="dist"
BINARY_NAME="ugudu"

echo "Building Ugudu v${VERSION} for all platforms..."

# Clean dist directory
rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

# Build matrix
PLATFORMS=(
    "darwin/amd64"
    "darwin/arm64"
    "linux/amd64"
    "linux/arm64"
    "windows/amd64"
)

for PLATFORM in "${PLATFORMS[@]}"; do
    GOOS="${PLATFORM%/*}"
    GOARCH="${PLATFORM#*/}"

    OUTPUT_NAME="${BINARY_NAME}"
    if [ "$GOOS" = "windows" ]; then
        OUTPUT_NAME="${OUTPUT_NAME}.exe"
    fi

    BUILD_DIR="${DIST_DIR}/${BINARY_NAME}_${VERSION}_${GOOS}_${GOARCH}"
    mkdir -p "$BUILD_DIR"

    echo "Building for ${GOOS}/${GOARCH}..."

    GOOS="$GOOS" GOARCH="$GOARCH" go build \
        -ldflags "-s -w -X main.Version=${VERSION}" \
        -o "${BUILD_DIR}/${OUTPUT_NAME}" \
        ./cmd/ugudu

    # Copy README and LICENSE
    cp README.md "${BUILD_DIR}/"
    [ -f LICENSE ] && cp LICENSE "${BUILD_DIR}/"

    # Create archive
    cd "$DIST_DIR"
    if [ "$GOOS" = "windows" ]; then
        zip -r "${BINARY_NAME}_${VERSION}_${GOOS}_${GOARCH}.zip" \
            "${BINARY_NAME}_${VERSION}_${GOOS}_${GOARCH}"
    else
        tar -czvf "${BINARY_NAME}_${VERSION}_${GOOS}_${GOARCH}.tar.gz" \
            "${BINARY_NAME}_${VERSION}_${GOOS}_${GOARCH}"
    fi
    cd ..

    # Cleanup build directory
    rm -rf "$BUILD_DIR"
done

# Generate checksums
echo "Generating checksums..."
cd "$DIST_DIR"
shasum -a 256 *.tar.gz *.zip > checksums.txt
cd ..

echo ""
echo "Build complete! Archives in ${DIST_DIR}/"
ls -la "$DIST_DIR"

echo ""
echo "Checksums:"
cat "${DIST_DIR}/checksums.txt"

echo ""
echo "Next steps:"
echo "  1. Create GitHub release: gh release create v${VERSION} ${DIST_DIR}/*"
echo "  2. Update Homebrew formula SHA256 values"
echo "  3. Update Chocolatey package checksum"
echo "  4. Publish to npm: npm publish"
