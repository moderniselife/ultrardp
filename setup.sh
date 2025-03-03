#!/bin/bash

# UltraRDP Setup Script
# This script prepares the environment for running UltraRDP

set -e  # Exit immediately if a command exits with a non-zero status

echo "=== UltraRDP Setup ==="

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed or not in PATH"
    echo "Please install Go from https://golang.org/dl/"
    exit 1
fi

# Check Go version
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
GO_MAJOR=$(echo $GO_VERSION | cut -d. -f1)
GO_MINOR=$(echo $GO_VERSION | cut -d. -f2)

if [ "$GO_MAJOR" -lt 1 ] || ([ "$GO_MAJOR" -eq 1 ] && [ "$GO_MINOR" -lt 16 ]); then
    echo "Warning: UltraRDP recommends Go 1.16 or newer. You have Go $GO_VERSION"
    read -p "Continue anyway? (y/n): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Make sure the script is executable
chmod +x "$(dirname "$0")/start.sh" 2>/dev/null || true

# Check for required system libraries based on OS
case "$(uname -s)" in
    Darwin*)
        echo "Detected macOS system"
        # Check for required macOS libraries
        if ! pkg-config --exists x11; then
            echo "Note: X11 development libraries not found. These may be required for screen capture."
            echo "Consider installing XQuartz if you encounter issues."
        fi
        ;;
    Linux*)
        echo "Detected Linux system"
        # Check for X11 development libraries on Linux
        if ! pkg-config --exists x11; then
            echo "Warning: X11 development libraries not found. These are required for screen capture."
            echo "On Ubuntu/Debian, install with: sudo apt-get install libx11-dev"
            echo "On Fedora/RHEL, install with: sudo dnf install libX11-devel"
        fi
        ;;
    CYGWIN*|MINGW*|MSYS*)
        echo "Detected Windows system"
        # Windows-specific checks could go here
        ;;
    *)
        echo "Unknown operating system. Setup may not be complete."
        ;;
esac

# Build the application
echo "Building UltraRDP..."
go build -o ultrardp main.go

if [ $? -eq 0 ]; then
    echo "\nSetup completed successfully!"
    echo "Run './start.sh' to start UltraRDP"
    echo "Run './start.sh --help' for more options"
else
    echo "\nBuild failed. Please check the error messages above."
    exit 1
fi