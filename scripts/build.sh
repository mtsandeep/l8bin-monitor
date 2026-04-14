#!/bin/bash
# Build Script for Unix/Linux/macOS
# Usage: ./scripts/build.sh [version]

VERSION=${1:-"dev"}
DIST_DIR="dist"
PLATFORMS=("linux/amd64" "linux/arm64" "darwin/amd64" "darwin/arm64" "windows/amd64")

mkdir -p $DIST_DIR

LDFLAGS="-s -w -X 'main.Version=$VERSION'"

echo "🚀 Starting Litebin Monitor Build (Version: $VERSION)"

for PLATFORM in "${PLATFORMS[@]}"; do
    IFS="/" read -r GOOS GOARCH <<< "$PLATFORM"
    
    BINARY_NAME="litebin-monitor-$GOOS-$GOARCH"
    if [ "$GOOS" == "windows" ]; then BINARY_NAME+=".exe"; fi
    
    OUTPUT_PATH="$DIST_DIR/$BINARY_NAME"
    
    echo -n "📦 Building for $GOOS/$GOARCH... "
    
    GOOS=$GOOS GOARCH=$GOARCH CGO_ENABLED=0 go build -ldflags "$LDFLAGS" -o "$OUTPUT_PATH" .
    
    if [ $? -eq 0 ]; then
        echo -e "\e[32m[OK]\e[0m"
        
        # Try UPX compression if available
        if command -v upx > /dev/null; then
            echo -n "  ✨ Compressing with UPX... "
            upx --best "$OUTPUT_PATH" > /dev/null
            echo -e "\e[33m[Done]\e[0m"
        fi
    else
        echo -e "\e[31m[FAILED]\e[0m"
    fi
done

echo "✅ Build complete! Binaries are in the '$DIST_DIR' folder."
