#!/bin/bash

set -e

# Colors for pretty output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Parse command line arguments
parse_args() {
  # Default values
  SPECIFIED_VERSION=""

  # Process command line arguments
  while [[ $# -gt 0 ]]; do
    case "$1" in
      -v|--version)
        SPECIFIED_VERSION="$2"
        shift 2
        ;;
      -h|--help)
        show_help
        exit 0
        ;;
      *)
        error "Unknown option: $1"
        show_help
        exit 1
        ;;
    esac
  done

  if [ -n "$SPECIFIED_VERSION" ]; then
    info "User specified version: $SPECIFIED_VERSION"
  fi
}

# Display help
show_help() {
  echo "Monitorly Probe Installer"
  echo
  echo "Usage: $0 [options]"
  echo
  echo "Options:"
  echo "  -v, --version VERSION    Install specific version"
  echo "  -h, --help               Show this help message"
  echo
}

# GitHub repository details
REPO_OWNER="monitorly-app"
REPO_NAME="probe"
GITHUB_API="https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest"

# Installation paths
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="${HOME}/.monitorly"
SERVICE_DIR=""

# Print with colors
info() {
  echo -e "${BLUE}INFO:${NC} $1"
}

success() {
  echo -e "${GREEN}SUCCESS:${NC} $1"
}

warning() {
  echo -e "${YELLOW}WARNING:${NC} $1"
}

error() {
  echo -e "${RED}ERROR:${NC} $1"
  exit 1
}

# Check for required commands
check_dependencies() {
  info "Checking dependencies..."

  for cmd in curl grep cut tr uname mktemp; do
    if ! command -v ${cmd} >/dev/null 2>&1; then
      error "Required command '${cmd}' not found."
    fi
  done

  # Check for sudo or root
  if [ "$(id -u)" -ne 0 ]; then
    if ! command -v sudo >/dev/null 2>&1; then
      error "This script requires sudo privileges to install system-wide. Please run as root or install sudo."
    fi
    USE_SUDO=true
  else
    USE_SUDO=false
  fi
}

# Detect Linux distribution and architecture
detect_platform() {
  info "Detecting platform..."

  # Verify we're running on Linux
  OS_TYPE=$(uname -s)
  if [ "$OS_TYPE" != "Linux" ]; then
    error "This installer only supports Linux. Detected OS: $OS_TYPE"
  fi

  # Detect Linux distribution
  if [ -f /etc/os-release ]; then
    . /etc/os-release
    DISTRO_NAME=$ID
    DISTRO_VERSION=$VERSION_ID
    DISTRO_FAMILY=""

    # Determine the distribution family
    case "$DISTRO_NAME" in
      rhel|centos|fedora|rocky|almalinux)
        DISTRO_FAMILY="redhat"
        ;;
      debian|ubuntu|linuxmint|pop)
        DISTRO_FAMILY="debian"
        ;;
      *)
        DISTRO_FAMILY="other"
        ;;
    esac
  else
    DISTRO_NAME="unknown"
    DISTRO_VERSION="unknown"
    DISTRO_FAMILY="other"
  fi

  # Detect init system
  if command -v systemctl >/dev/null 2>&1; then
    INIT_SYSTEM="systemd"
    SERVICE_DIR="/etc/systemd/system"
  elif command -v service >/dev/null 2>&1; then
    INIT_SYSTEM="sysv"
    SERVICE_DIR="/etc/init.d"
  else
    warning "Could not detect init system. Service installation will be skipped."
    INIT_SYSTEM="unknown"
  fi

  # Detect architecture
  ARCH=$(uname -m)
  case "$ARCH" in
    x86_64)
      ARCH="amd64"
      ;;
    aarch64|arm64)
      ARCH="arm64"
      ;;
    *)
      error "Unsupported architecture: $ARCH"
      ;;
  esac

  info "Detected distribution: ${DISTRO_NAME} ${DISTRO_VERSION} (${DISTRO_FAMILY})"
  info "Architecture: ${ARCH}, Init system: ${INIT_SYSTEM}"
}

# Get the latest release version and download URL
get_latest_release() {
  # If version is specified, use it directly
  if [ -n "$SPECIFIED_VERSION" ]; then
    VERSION="$SPECIFIED_VERSION"
    info "Using specified version: ${VERSION}"

    # Construct the download URL directly
    DOWNLOAD_URL="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/v${VERSION}/monitorly-probe-${VERSION}-linux-${ARCH}"
    info "Download URL: ${DOWNLOAD_URL}"
    return
  fi

  info "Fetching latest release information..."

  # First try to get the latest release from the GitHub API
  if RELEASE_DATA=$(curl -s -f ${GITHUB_API}); then
    # Extract version
    VERSION=$(echo "${RELEASE_DATA}" | grep -o '"tag_name": *"[^"]*"' | grep -o '[^"]*$')
    VERSION="${VERSION#v}" # Remove leading 'v'

    if [ -z "$VERSION" ]; then
      warning "Could not determine the latest version from GitHub API. Using fallback method..."
    else
      info "Latest version: ${VERSION}"

      # Build asset name pattern (standalone binary)
      ASSET_PATTERN="monitorly-probe-${VERSION}-linux-${ARCH}"

      # Find the download URL
      if DOWNLOAD_URL=$(echo "${RELEASE_DATA}" | grep -o "\"browser_download_url\": *\"[^\"]*${ASSET_PATTERN}[^\"]*\"" | grep -o 'http[^\"]*'); then
        info "Download URL: ${DOWNLOAD_URL}"
        return
      else
        warning "Could not find download URL for linux-${ARCH} via API. Using fallback method..."
      fi
    fi
  else
    warning "Failed to fetch latest release data from GitHub API. Using fallback method..."
  fi

  # Fallback: Use a hardcoded version as a last resort
  # This should be updated periodically when releasing new versions
  FALLBACK_VERSION="0.1.0"
  VERSION="${FALLBACK_VERSION}"
  info "Using fallback version: ${VERSION}"

  # Construct the download URL directly
  DOWNLOAD_URL="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/v${VERSION}/monitorly-probe-${VERSION}-linux-${ARCH}"
  info "Fallback download URL: ${DOWNLOAD_URL}"
}

# Download and install the binary
download_and_install() {
  info "Downloading Monitorly Probe ${VERSION}..."

  TEMP_DIR=$(mktemp -d)
  TEMP_FILE="${TEMP_DIR}/monitorly-probe"

  if ! curl -L -s "${DOWNLOAD_URL}" -o "${TEMP_FILE}"; then
    error "Failed to download the binary."
  fi

  info "Installing to ${INSTALL_DIR}..."

  # Make executable and move to install dir
  chmod +x "${TEMP_FILE}"

  if [ "$USE_SUDO" = true ]; then
    if ! sudo mkdir -p "${INSTALL_DIR}"; then
      error "Failed to create installation directory."
    fi
    if ! sudo mv "${TEMP_FILE}" "${INSTALL_DIR}/monitorly-probe"; then
      error "Failed to move binary to installation directory."
    fi
  else
    if ! mkdir -p "${INSTALL_DIR}"; then
      error "Failed to create installation directory."
    fi
    if ! mv "${TEMP_FILE}" "${INSTALL_DIR}/monitorly-probe"; then
      error "Failed to move binary to installation directory."
    fi
  fi

  # Clean up temp directory
  rm -rf "${TEMP_DIR}"

  success "Monitorly Probe v${VERSION} installed to ${INSTALL_DIR}/monitorly-probe"
}

# Download the example config file
download_config() {
  info "Setting up configuration directory..."

  mkdir -p "${CONFIG_DIR}"

  # Try to get the example config
  # First try with the exact version tag
  CONFIG_URL="https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/v${VERSION}/config.yaml.example"
  info "Downloading example config from: ${CONFIG_URL}"

  if curl -s -f "${CONFIG_URL}" -o "${CONFIG_DIR}/config.yaml"; then
    success "Configuration file saved to ${CONFIG_DIR}/config.yaml"
  else
    # Fall back to the main branch if version-specific file isn't available
    CONFIG_URL="https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/main/config.yaml.example"
    info "Version-specific config not found. Trying from main branch: ${CONFIG_URL}"

    if curl -s -f "${CONFIG_URL}" -o "${CONFIG_DIR}/config.yaml"; then
      success "Configuration file saved to ${CONFIG_DIR}/config.yaml from main branch"
    else
      warning "Could not download example config. Creating a minimal one instead."
      cat > "${CONFIG_DIR}/config.yaml" << EOF
# Monitorly Probe Configuration

# Optional machine name for identifying this server in metrics
# If not specified, the system hostname will be used
# machine_name: "my-server"

collection:
  # CPU metrics collection settings
  cpu:
    enabled: true
    interval: 30s

  # RAM metrics collection settings
  ram:
    enabled: true
    interval: 30s

  # Disk metrics collection settings
  disk:
    enabled: true
    interval: 60s
    mount_points:
      - path: "/"
        label: "root"
        collect_usage: true
        collect_percent: true

# Sender configuration
sender:
  target: "log_file"  # Change to "api" to send to API
  send_interval: 5m

# API configuration (required if sender.target is "api")
api:
  url: "https://api.monitorly.io/metrics"
  project_id: "YOUR_PROJECT_ID"
  application_token: "YOUR_APP_TOKEN"

# Log file configuration (used if sender.target is "log_file")
log_file:
  path: "${CONFIG_DIR}/metrics.log"

# Application logging configuration
logging:
  file_path: "${CONFIG_DIR}/monitorly.log"
EOF
      success "Created default configuration file at ${CONFIG_DIR}/config.yaml"
    fi
  fi

  # Create logging directories
  mkdir -p "${CONFIG_DIR}/logs"
}

# Set up system service based on the detected init system
setup_service() {
  info "Setting up system service..."

  case "$INIT_SYSTEM" in
    systemd)
      info "Creating systemd service..."
      SERVICE_NAME="monitorly-probe.service"
      SERVICE_FILE="${SERVICE_DIR}/${SERVICE_NAME}"

      # Create service file content
      SERVICE_CONTENT="[Unit]
Description=Monitorly Probe - System Monitoring Agent
After=network.target

[Service]
ExecStart=${INSTALL_DIR}/monitorly-probe
Restart=always
# Consider creating a dedicated user for security
# User=monitorly
# Group=monitorly

[Install]
WantedBy=multi-user.target"

      # Write service file
      if [ "$USE_SUDO" = true ]; then
        echo "${SERVICE_CONTENT}" | sudo tee "${SERVICE_FILE}" > /dev/null
        sudo systemctl daemon-reload
        sudo systemctl enable "${SERVICE_NAME}"
        success "Created systemd service. You can start it with: sudo systemctl start ${SERVICE_NAME}"
      else
        echo "${SERVICE_CONTENT}" > "${SERVICE_FILE}"
        systemctl daemon-reload
        systemctl enable "${SERVICE_NAME}"
        success "Created systemd service. You can start it with: systemctl start ${SERVICE_NAME}"
      fi
      ;;

    sysv)
      info "Creating SysV init script..."
      INIT_SCRIPT="${SERVICE_DIR}/monitorly-probe"

      # Create init script content
      INIT_SCRIPT_CONTENT="#!/bin/sh
### BEGIN INIT INFO
# Provides:          monitorly-probe
# Required-Start:    \$network \$remote_fs \$syslog
# Required-Stop:     \$network \$remote_fs \$syslog
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: Monitorly Probe - System Monitoring Agent
# Description:       Monitorly Probe collects system metrics and sends them to a central API.
### END INIT INFO

PATH=/sbin:/usr/sbin:/bin:/usr/bin
DESC=\"Monitorly Probe\"
NAME=monitorly-probe
DAEMON=${INSTALL_DIR}/monitorly-probe
PIDFILE=/var/run/\$NAME.pid

case \"\$1\" in
  start)
    echo \"Starting \$DESC\"
    start-stop-daemon --start --background --make-pidfile --pidfile \$PIDFILE --exec \$DAEMON
    ;;
  stop)
    echo \"Stopping \$DESC\"
    start-stop-daemon --stop --pidfile \$PIDFILE --retry=TERM/30/KILL/5
    rm -f \$PIDFILE
    ;;
  restart)
    \$0 stop
    sleep 1
    \$0 start
    ;;
  status)
    if [ -f \$PIDFILE ]; then
      PID=\$(cat \$PIDFILE)
      if ps -p \$PID > /dev/null; then
        echo \"\$DESC is running (PID: \$PID)\"
        exit 0
      else
        echo \"\$DESC is not running (stale PID file)\"
        exit 1
      fi
    else
      echo \"\$DESC is not running\"
      exit 3
    fi
    ;;
  *)
    echo \"Usage: \$0 {start|stop|restart|status}\"
    exit 1
    ;;
esac

exit 0"

      # Write init script
      if [ "$USE_SUDO" = true ]; then
        echo "${INIT_SCRIPT_CONTENT}" | sudo tee "${INIT_SCRIPT}" > /dev/null
        sudo chmod +x "${INIT_SCRIPT}"
        if command -v update-rc.d >/dev/null 2>&1; then
          sudo update-rc.d monitorly-probe defaults
        elif command -v chkconfig >/dev/null 2>&1; then
          sudo chkconfig --add monitorly-probe
        fi
        success "Created SysV init script. You can start it with: sudo service monitorly-probe start"
      else
        echo "${INIT_SCRIPT_CONTENT}" > "${INIT_SCRIPT}"
        chmod +x "${INIT_SCRIPT}"
        if command -v update-rc.d >/dev/null 2>&1; then
          update-rc.d monitorly-probe defaults
        elif command -v chkconfig >/dev/null 2>&1; then
          chkconfig --add monitorly-probe
        fi
        success "Created SysV init script. You can start it with: service monitorly-probe start"
      fi
      ;;

    *)
      warning "Automatic service installation is not supported for your system."
      info "You can run the probe manually with: ${INSTALL_DIR}/monitorly-probe"
      ;;
  esac
}

# Set up distribution-specific configurations
setup_distro_specific() {
  case "$DISTRO_FAMILY" in
    redhat)
      info "Setting up Red Hat family specific configurations..."

      # For SELinux systems, set the proper context
      if command -v sestatus >/dev/null 2>&1 && sestatus | grep -q "SELinux status: *enabled"; then
        if [ "$USE_SUDO" = true ]; then
          info "SELinux detected, setting appropriate context"
          sudo chcon -t bin_t "${INSTALL_DIR}/monitorly-probe"
        fi
      fi

      # Create logrotate configuration
      if [ -d /etc/logrotate.d ]; then
        LOGROTATE_CONFIG="/etc/logrotate.d/monitorly-probe"
        LOGROTATE_CONTENT="${CONFIG_DIR}/logs/*.log {
    weekly
    missingok
    rotate 7
    compress
    delaycompress
    notifempty
    create 0640 root root
}"
        if [ "$USE_SUDO" = true ]; then
          echo "${LOGROTATE_CONTENT}" | sudo tee "${LOGROTATE_CONFIG}" > /dev/null
          success "Created logrotate configuration"
        fi
      fi
      ;;

    debian)
      info "Setting up Debian family specific configurations..."

      # Create logrotate configuration
      if [ -d /etc/logrotate.d ]; then
        LOGROTATE_CONFIG="/etc/logrotate.d/monitorly-probe"
        LOGROTATE_CONTENT="${CONFIG_DIR}/logs/*.log {
    weekly
    missingok
    rotate 7
    compress
    delaycompress
    notifempty
    create 0640 root root
}"
        if [ "$USE_SUDO" = true ]; then
          echo "${LOGROTATE_CONTENT}" | sudo tee "${LOGROTATE_CONFIG}" > /dev/null
          success "Created logrotate configuration"
        fi
      fi
      ;;

    *)
      info "No distribution-specific configurations to apply"
      ;;
  esac
}

# Display post-installation instructions
show_instructions() {
  echo
  echo -e "${GREEN}════════════════════════════════════════════════════════${NC}"
  echo -e "${GREEN}       Monitorly Probe Installation Complete!${NC}"
  echo -e "${GREEN}════════════════════════════════════════════════════════${NC}"
  echo
  echo -e "The probe has been installed to: ${BLUE}${INSTALL_DIR}/monitorly-probe${NC}"
  echo -e "Configuration file: ${BLUE}${CONFIG_DIR}/config.yaml${NC}"
  echo
  echo -e "${YELLOW}IMPORTANT:${NC} Before starting the service, please review and edit your configuration:"
  echo -e "  ${BLUE}vim ${CONFIG_DIR}/config.yaml${NC}"
  echo
  echo -e "Specifically, if you want to send metrics to the Monitorly API, edit these settings:"
  echo -e "  1. Change ${BLUE}sender.target${NC} to ${BLUE}\"api\"${NC}"
  echo -e "  2. Set your ${BLUE}api.project_id${NC} and ${BLUE}api.application_token${NC}"
  echo

  # Display service-specific instructions
  case "$INIT_SYSTEM" in
    systemd)
      echo -e "To manage the Monitorly Probe service:"
      echo -e "  ${BLUE}sudo systemctl start monitorly-probe${NC}   # Start the service"
      echo -e "  ${BLUE}sudo systemctl stop monitorly-probe${NC}    # Stop the service"
      echo -e "  ${BLUE}sudo systemctl status monitorly-probe${NC}  # Check status"
      echo -e "  ${BLUE}sudo journalctl -u monitorly-probe${NC}     # View logs"
      ;;
    sysv)
      echo -e "To manage the Monitorly Probe service:"
      echo -e "  ${BLUE}sudo service monitorly-probe start${NC}    # Start the service"
      echo -e "  ${BLUE}sudo service monitorly-probe stop${NC}     # Stop the service"
      echo -e "  ${BLUE}sudo service monitorly-probe status${NC}   # Check status"
      echo -e "  ${BLUE}tail -f ${CONFIG_DIR}/logs/monitorly.log${NC} # View logs"
      ;;
    *)
      echo -e "To run Monitorly Probe manually:"
      echo -e "  ${BLUE}${INSTALL_DIR}/monitorly-probe${NC}"
      echo -e "Log files will be created in: ${BLUE}${CONFIG_DIR}/logs/${NC}"
      ;;
  esac

  echo
  echo -e "Visit ${BLUE}https://github.com/monitorly-app/probe${NC} for more information."
  echo -e "${GREEN}═════════════════════════════════════════════════════════${NC}"
}

# Main installation process
main() {
  echo -e "${GREEN}═════════════════════════════════════════════════════════${NC}"
  echo -e "${GREEN}                Monitorly Probe Installer                ${NC}"
  echo -e "${GREEN}═════════════════════════════════════════════════════════${NC}"
  echo

  parse_args "$@"
  check_dependencies
  detect_platform
  get_latest_release
  download_and_install
  download_config
  setup_service
  setup_distro_specific
  show_instructions
}

# Pass all arguments to main
main "$@"