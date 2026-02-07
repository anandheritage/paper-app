#!/bin/bash
set -euo pipefail

# ============================================================
# EC2 Setup Script for Paper App
# Run this ONCE on a fresh Ubuntu 22.04/24.04 EC2 instance
#
# Usage:
#   chmod +x deploy/setup-ec2.sh
#   ./deploy/setup-ec2.sh
# ============================================================

echo "=== Paper App — EC2 Setup ==="

# ─── 1. System updates ───
echo "→ Updating system packages..."
sudo apt-get update -y
sudo apt-get upgrade -y

# ─── 2. Install Docker ───
echo "→ Installing Docker..."
if ! command -v docker &>/dev/null; then
    curl -fsSL https://get.docker.com | sudo sh
    sudo usermod -aG docker "$USER"
    echo "   Docker installed. You may need to log out and back in for group changes."
else
    echo "   Docker already installed."
fi

# ─── 3. Install Docker Compose plugin ───
echo "→ Installing Docker Compose..."
if ! docker compose version &>/dev/null 2>&1; then
    sudo apt-get install -y docker-compose-plugin
else
    echo "   Docker Compose already installed."
fi

# ─── 4. Install useful tools ───
echo "→ Installing utilities..."
sudo apt-get install -y git htop curl jq unzip

# ─── 5. Increase vm.max_map_count for OpenSearch ───
echo "→ Configuring kernel for OpenSearch..."
if ! grep -q "vm.max_map_count=262144" /etc/sysctl.conf; then
    echo "vm.max_map_count=262144" | sudo tee -a /etc/sysctl.conf
    sudo sysctl -w vm.max_map_count=262144
fi

# ─── 6. Increase file descriptor limits ───
if ! grep -q "* soft nofile 65536" /etc/security/limits.conf; then
    echo "* soft nofile 65536" | sudo tee -a /etc/security/limits.conf
    echo "* hard nofile 65536" | sudo tee -a /etc/security/limits.conf
fi

# ─── 7. Set up app directory ───
APP_DIR="/opt/paper"
echo "→ Creating app directory at ${APP_DIR}..."
sudo mkdir -p "$APP_DIR"
sudo chown "$USER:$USER" "$APP_DIR"

# ─── 8. Create swap (useful for t3.medium with 4GB RAM) ───
if [ ! -f /swapfile ]; then
    echo "→ Creating 2GB swap file..."
    sudo fallocate -l 2G /swapfile
    sudo chmod 600 /swapfile
    sudo mkswap /swapfile
    sudo swapon /swapfile
    echo "/swapfile swap swap defaults 0 0" | sudo tee -a /etc/fstab
fi

echo ""
echo "=== Setup complete! ==="
echo ""
echo "Next steps:"
echo "  1. Clone your repo:     cd /opt/paper && git clone <your-repo-url> ."
echo "  2. Copy env file:       cp .env.production.example .env.production"
echo "  3. Edit env file:       nano .env.production"
echo "  4. Deploy:              ./deploy/deploy.sh"
echo ""
echo "If you just installed Docker, log out and back in first:"
echo "  exit && ssh <your-instance>"
