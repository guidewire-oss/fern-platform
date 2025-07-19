#!/bin/bash
# Pre-removal script for Fern Platform

set -e

# Stop the service if it's running
if command -v systemctl >/dev/null 2>&1; then
    if systemctl is-active --quiet fern-platform; then
        systemctl stop fern-platform
    fi
    
    if systemctl is-enabled --quiet fern-platform; then
        systemctl disable fern-platform
    fi
fi

echo "Fern Platform service has been stopped."