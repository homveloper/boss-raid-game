#!/bin/bash
# Build script for the Text-Based Boss Raid Game
# This script builds the game for various operating systems and architectures

# Default values
OS="all"
ARCH="all"
OUTPUT="./bin"
APP_NAME="boss-raid-game"

# Help function
show_help() {
    echo "Usage: $0 [options]"
    echo "Options:"
    echo "  -o, --os OS        Operating system to build for (windows, darwin, linux, all)"
    echo "  -a, --arch ARCH    Architecture to build for (386, amd64, arm, arm64, all)"
    echo "  -d, --output DIR   Output directory for binaries (default: ./bin)"
    echo "  -h, --help         Show this help message"
    echo
    echo "Examples:"
    echo "  $0                  Build for all supported platforms and architectures"
    echo "  $0 -o windows -a amd64  Build only for Windows 64-bit"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    key="$1"
    case $key in
        -o|--os)
            OS="$2"
            shift
            shift
            ;;
        -a|--arch)
            ARCH="$2"
            shift
            shift
            ;;
        -d|--output)
            OUTPUT="$2"
            shift
            shift
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

# Create output directory if it doesn't exist
mkdir -p "$OUTPUT"
echo "Output directory: $OUTPUT"

# Define supported platforms
declare -A WINDOWS_ARCHS=( ["386"]=1 ["amd64"]=1 ["arm"]=1 ["arm64"]=1 )
declare -A DARWIN_ARCHS=( ["amd64"]=1 ["arm64"]=1 )
declare -A LINUX_ARCHS=( ["386"]=1 ["amd64"]=1 ["arm"]=1 ["arm64"]=1 )

# Function to build for a specific OS and architecture
build_binary() {
    local target_os=$1
    local target_arch=$2
    
    # Set extension based on OS
    local extension=""
    if [[ "$target_os" == "windows" ]]; then
        extension=".exe"
    fi
    
    local output_file="${OUTPUT}/${APP_NAME}_${target_os}_${target_arch}${extension}"
    
    echo "Building for $target_os/$target_arch..."
    
    # Set environment variables for cross-compilation
    export GOOS=$target_os
    export GOARCH=$target_arch
    
    # For ARM, set GOARM to 7 (ARMv7) as a default
    if [[ "$target_arch" == "arm" ]]; then
        export GOARM=7
    fi
    
    # Build the binary
    go build -o "$output_file" -ldflags="-s -w" .
    
    if [ $? -eq 0 ]; then
        echo -e "\e[32mSuccessfully built: $output_file\e[0m"
    else
        echo -e "\e[31mFailed to build for $target_os/$target_arch\e[0m"
    fi
}

# Build for all specified platforms
if [[ "$OS" == "all" || "$OS" == "windows" ]]; then
    if [[ "$ARCH" == "all" ]]; then
        for arch in "${!WINDOWS_ARCHS[@]}"; do
            build_binary "windows" "$arch"
        done
    elif [[ -n "${WINDOWS_ARCHS[$ARCH]}" ]]; then
        build_binary "windows" "$ARCH"
    else
        echo -e "\e[33mArchitecture $ARCH is not supported for windows\e[0m"
    fi
fi

if [[ "$OS" == "all" || "$OS" == "darwin" ]]; then
    if [[ "$ARCH" == "all" ]]; then
        for arch in "${!DARWIN_ARCHS[@]}"; do
            build_binary "darwin" "$arch"
        done
    elif [[ -n "${DARWIN_ARCHS[$ARCH]}" ]]; then
        build_binary "darwin" "$ARCH"
    else
        echo -e "\e[33mArchitecture $ARCH is not supported for darwin\e[0m"
    fi
fi

if [[ "$OS" == "all" || "$OS" == "linux" ]]; then
    if [[ "$ARCH" == "all" ]]; then
        for arch in "${!LINUX_ARCHS[@]}"; do
            build_binary "linux" "$arch"
        done
    elif [[ -n "${LINUX_ARCHS[$ARCH]}" ]]; then
        build_binary "linux" "$ARCH"
    else
        echo -e "\e[33mArchitecture $ARCH is not supported for linux\e[0m"
    fi
fi

echo -e "\e[32mBuild process completed!\e[0m"

# Make the script executable if it's not already
chmod +x "$0"
