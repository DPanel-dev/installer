#!/bin/bash
# Docker Installation Script for Alpine Linux
# Uses apk package manager

set -e

# Check if Docker is already installed
if command -v docker >/dev/null 2>&1; then
    echo "Docker is already installed"
    docker --version
    exit 0
fi

echo "Installing Docker on Alpine Linux..."

# Install Docker via apk
apk add --no-cache --update docker

# Create docker config directory
mkdir -p /etc/docker

# Start Docker service
if command -v rc-service >/dev/null 2>&1; then
    rc-service docker start
    rc-update add docker default
fi

# Verify installation
if command -v docker >/dev/null 2>&1; then
    echo "Docker installed successfully!"
    docker --version
    exit 0
else
    echo "ERROR: Docker installation failed"
    exit 1
fi
