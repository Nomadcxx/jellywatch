#!/bin/bash

# Jellywatch Installation Script

set -e

INSTALL_DIR="/usr/local/bin"
SERVICE_DIR="/etc/systemd/system"
CONFIG_DIR="/home/$SUDO_USER/.config/jellywatch"

echo "Installing jellywatch..."

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root"
    exit 1
fi

# Create config directory
mkdir -p "$CONFIG_DIR"

# Copy binaries
echo "Copying binaries to $INSTALL_DIR..."
cp jellywatch "$INSTALL_DIR/"
cp jellywatchd "$INSTALL_DIR/"
chmod +x "$INSTALL_DIR/jellywatch"
chmod +x "$INSTALL_DIR/jellywatchd"

# Install systemd service
echo "Installing systemd service..."
cp systemd/jellywatchd.service "$SERVICE_DIR/"
sed -i "s/%USERNAME%/$SUDO_USER/g" "$SERVICE_DIR/jellywatchd.service"
systemctl daemon-reload

echo ""
echo "Installation complete!"
echo ""
echo "Next steps:"
echo "  1. Configure jellywatch: jellywatch"
echo "  2. Add watch directories to ~/.config/jellywatch/config.toml"
echo "  3. Enable daemon: sudo systemctl enable jellywatchd"
echo "  4. Start daemon: sudo systemctl start jellywatchd"
echo ""
echo "For usage, run: jellywatch --help"
