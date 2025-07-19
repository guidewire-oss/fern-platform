#!/bin/bash
# Post-installation script for Fern Platform

set -e

# Create user and group if they don't exist
if ! getent group fern-platform >/dev/null 2>&1; then
    groupadd --system fern-platform
fi

if ! getent passwd fern-platform >/dev/null 2>&1; then
    useradd --system --gid fern-platform --home-dir /var/lib/fern-platform --shell /usr/sbin/nologin fern-platform
fi

# Create directories
mkdir -p /var/lib/fern-platform
mkdir -p /var/log/fern-platform
mkdir -p /etc/fern-platform

# Set permissions
chown -R fern-platform:fern-platform /var/lib/fern-platform
chown -R fern-platform:fern-platform /var/log/fern-platform
chown -R fern-platform:fern-platform /etc/fern-platform

# Reload systemd
if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload
fi

echo "Fern Platform has been installed successfully!"
echo ""
echo "To start the service:"
echo "  systemctl start fern-platform"
echo ""
echo "To enable automatic startup:"
echo "  systemctl enable fern-platform"
echo ""
echo "Configuration file: /etc/fern-platform/config.yaml"