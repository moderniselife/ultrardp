#!/bin/bash

# UltraRDP Start Script
# This script launches the UltraRDP application in either server or client mode

# Function to display help message
show_help() {
    echo "UltraRDP - High Performance Remote Desktop Protocol"
    echo ""
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -s, --server          Run in server mode"
    echo "  -c, --client          Run in client mode (default)"
    echo "  -a, --address ADDR    Specify address to connect to or listen on (default: localhost:8000)"
    echo "  -h, --help            Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 --server                   # Start as server on default port"
    echo "  $0 --server --address 0.0.0.0:8000  # Start server listening on all interfaces"
    echo "  $0 --client --address 192.168.1.5:8000  # Connect to a specific server"
}

# Check if the UltraRDP binary exists
if [ ! -f "./ultrardp" ]; then
    echo "Error: UltraRDP binary not found."
    echo "Please run './setup.sh' first to build the application."
    exit 1
fi

# Default values
MODE="client"
ADDRESS="localhost:8000"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -s|--server)
            MODE="server"
            shift
            ;;
        -c|--client)
            MODE="client"
            shift
            ;;
        -a|--address)
            if [ -z "$2" ] || [[ "$2" == -* ]]; then
                echo "Error: --address requires an argument."
                exit 1
            fi
            ADDRESS="$2"
            shift 2
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

# Set up signal handling to gracefully exit
trap "echo -e '\nStopping UltraRDP...'; exit 0" INT TERM

# Run UltraRDP with the appropriate options
if [ "$MODE" == "server" ]; then
    echo "Starting UltraRDP Server on $ADDRESS"
    ./ultrardp --server --address "$ADDRESS"
else
    echo "Starting UltraRDP Client, connecting to $ADDRESS"
    ./ultrardp --address "$ADDRESS"
fi