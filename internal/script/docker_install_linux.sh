#!/bin/bash
# Docker Installation Script for Standard Linux
# Uses get.docker.com official script with Chinese mirror support

set -e

# Check if Docker is already installed
if command -v docker >/dev/null 2>&1; then
    echo "Docker is already installed"
    docker --version
    exit 0
fi

# Detect IP region for Chinese mirror selection
detect_region() {
    local region=""
    if command -v curl >/dev/null 2>&1; then
        region=$(curl -s --connect-timeout 3 -m 5 https://ipinfo.io/region 2>/dev/null || echo "")
    fi
    echo "$region"
}

# Select fastest Chinese mirror
select_mirror() {
    local mirrors=(
        "https://mirrors.aliyun.com/docker-ce"
        "https://mirrors.tencent.com/docker-ce"
        "https://mirrors.163.com/docker-ce"
        "https://mirrors.cnet.edu.cn/docker-ce"
    )

    local script_mirrors=(
        "https://get.docker.com"
        "https://testingcf.jsdelivr.net/gh/docker/docker-install@master/install.sh"
        "https://cdn.jsdelivr.net/gh/docker/docker-install@master/install.sh"
        "https://fastly.jsdelivr.net/gh/docker/docker-install@master/install.sh"
    )

    local min_delay=999999
    local selected_mirror=""

    for mirror in "${mirrors[@]}"; do
        local delay=$(curl -o /dev/null -s -m 2 -w "%{time_total}" "$mirror" 2>/dev/null || echo "999999")
        delay=$(echo "$delay * 1000" | bc 2>/dev/null || echo "999999")
        if [ "${delay%.*}" -lt "${min_delay%.*}" ]; then
            min_delay=$delay
            selected_mirror=$mirror
        fi
    done

    if [ -n "$selected_mirror" ]; then
        echo "Selected mirror: $selected_mirror (latency: ${min_delay}ms)"
        export DOWNLOAD_URL="$selected_mirror"
    fi

    # Download install script
    for script_url in "${script_mirrors[@]}"; do
        echo "Trying: $script_url"
        if curl -fsSL --connect-timeout 5 -m 10 "$script_url" -o /tmp/get-docker.sh 2>/dev/null; then
            echo "Downloaded from: $script_url"
            break
        fi
    done

    if [ ! -f /tmp/get-docker.sh ]; then
        echo "ERROR: Failed to download Docker install script"
        exit 1
    fi
}

# Main installation logic
main() {
    echo "Installing Docker on Linux..."

    # Check if running in China (optional optimization)
    local region=$(detect_region)
    if [ "$region" = "CN" ] || [ -n "$FORCE_CN_MIRROR" ]; then
        echo "Using Chinese mirrors for faster download..."
        select_mirror
    else
        echo "Using official Docker install script..."
        curl -fsSL https://get.docker.com -o /tmp/get-docker.sh
    fi

    # Execute installation
    sh /tmp/get-docker.sh

    # Clean up
    rm -f /tmp/get-docker.sh

    # Create docker config directory
    mkdir -p /etc/docker

    # Enable Docker service
    if command -v systemctl >/dev/null 2>&1; then
        systemctl enable docker
        systemctl start docker
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
}

main "$@"
