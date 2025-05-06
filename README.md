<p align="center"><a href="https://monitorly.io" target="_blank"><img src="https://raw.githubusercontent.com/monitorly-app/probe/master/logo.svg" width="400" alt="Monitorly Logo"></a></p>

<p align="center">
<a href="https://github.com/monitorly.app/probe"><img src="https://img.shields.io/badge/version-v0.1.0-blue" alt="Version"></a>
</p>

# Monitorly Probe

A lightweight server monitoring probe that collects system metrics and sends them to a central API or logs them to a file.

## Features

- Independent metric collection for CPU, RAM, and Disk usage
- Configurable collection intervals for each metric type
- Flexible disk monitoring with support for multiple mount points
- Machine identification for multi-server monitoring
- Metrics can be sent to a central API or logged to a local file
- Auto-reloading when config file changes
- Smart config file discovery
- Low resource footprint

## Supported Platforms

- Linux (amd64, arm64)
  - Debian-based distributions (Debian, Ubuntu, etc.)
  - RHEL-based distributions (Red Hat, CentOS, Fedora, Rocky Linux, etc.)

## Quick Installation

The easiest way to install Monitorly Probe is by using our installation script:

```bash
curl -sSL https://raw.githubusercontent.com/monitorly-app/probe/main/install.sh | bash
```

The installer will:
1. Detect your Linux distribution and architecture
2. Download the appropriate binary from the latest release
3. Install the probe to `/usr/local/bin/`
4. Set up a configuration file in `~/.monitorly/`
5. Create a system service appropriate for your distribution (systemd or SysV init)
6. Apply distribution-specific configurations (SELinux context, logrotate, etc.)
7. Provide instructions for completing setup

### Installation Options

You can specify a particular version to install:

```bash
# Install a specific version
curl -sSL https://raw.githubusercontent.com/monitorly-app/probe/main/install.sh | bash -s -- --version 1.0.0
```

Available options:
- `-v, --version VERSION` - Install a specific version
- `-h, --help` - Show help information

After installation, you'll need to edit the configuration file to suit your needs, particularly if you want to send metrics to the Monitorly API.

## Setup for Development

### Prerequisites

- Go 1.16 or later
- Git

### Installation

1. Clone the repository:

```bash
git clone https://github.com/monitorly-app/monitorly-probe.git
cd monitorly-probe
```

2. Install dependencies:

```bash
go mod download
```

3. Build the application:

```bash
go build -o bin/monitorly-probe ./cmd/probe
```

4. Create a configuration file (or use the example provided):

```bash
cp config.yaml.example config.yaml
```

5. Edit the configuration file to match your environment:

```bash
vim config.yaml
```

6. Run the application:

```bash
# The probe will automatically find your config file
./bin/monitorly-probe

# Or specify a custom config path
./bin/monitorly-probe -config /etc/monitorly/config.yaml
```

## Config File Discovery

The probe automatically searches for a configuration file in these locations (in order):

1. Path specified with the `-config` flag (if provided)
2. `~/.monitorly/config.yaml` (in user's home directory)
3. `config.yaml` (in the current working directory)
4. `configs/config.yaml` (in a configs subdirectory)
5. `/etc/monitorly/config.yaml` (system-wide configuration)

This means that during development, you can simply place a `config.yaml` file in the project directory, and the probe will find it automatically without any command-line arguments.

## Using as a System Service

The probe can be run as a system service using systemd, upstart, or other service managers. Here's an example systemd service file:

```ini
[Unit]
Description=Monitorly Probe
After=network.target

[Service]
ExecStart=/usr/local/bin/monitorly-probe -config /etc/monitorly/config.yaml
Restart=always
User=monitorly
Group=monitorly

[Install]
WantedBy=multi-user.target
```

## Releases and Versioning

The Monitorly Probe uses semantic versioning. Releases are created by tagging the repository with a version number (e.g., `v1.0.0`), which automatically triggers a GitHub Actions workflow to:

1. Build executables for multiple platforms (Linux, Windows, macOS)
2. Package executables with example configuration
3. Create a GitHub Release with the built artifacts

### Creating a Release

To create a new release:

1. Ensure all changes are committed and merged to the main branch
2. Run the release script with the new version number:
   ```bash
   ./scripts/release.sh 1.0.0
   ```
3. Push the tag to GitHub to trigger the build:
   ```bash
   git push origin v1.0.0
   ```

After the GitHub Actions workflow completes, the new release will be available on the GitHub Releases page with downloadable executables for all supported platforms.

### Release Artifacts

Each release includes the following artifacts for Linux platforms:
- Standalone executables for direct use
- Archive packages containing the executable and an example configuration file

Available architectures:
- amd64 (x86_64)
- arm64 (aarch64)

Choose the standalone executable if you:
- Want the simplest installation process
- Already have your own configuration file
- Prefer lightweight deployment

Choose the archive package if you:
- Are new to Monitorly Probe
- Want a configuration template to get started
- Need both files bundled together

## Configuration Auto-Reloading

The probe automatically monitors its configuration file for changes. When the file is modified, the probe will:

1. Detect the change
2. Load the new configuration
3. Gracefully shut down existing collectors and senders
4. Start new collectors and senders with the updated configuration

This allows you to modify the configuration without restarting the service manually.

## Configuration

The probe is configured using a YAML file. Here's an example configuration with all available options:

```yaml
# Optional machine name for identifying this server in metrics
# If not specified, the system hostname will be used
machine_name: "web-server-01"

collection:
  # CPU metrics collection settings
  cpu:
    enabled: true           # Enable/disable CPU metrics collection
    interval: 5s            # Collection interval (e.g., 5s, 1m, 1h)

  # RAM metrics collection settings
  ram:
    enabled: true           # Enable/disable RAM metrics collection
    interval: 10s           # Collection interval

  # Disk metrics collection settings
  disk:
    enabled: true           # Enable/disable disk metrics collection
    interval: 30s           # Collection interval
    mount_points:           # List of mount points to monitor
      - path: "/"           # Path to the mount point
        label: "root"       # User-friendly label for the mount point
        collect_usage: true  # Collect disk usage in bytes
        collect_percent: true # Collect disk usage as percentage
      - path: "/home"
        label: "home"
        collect_percent: true

# Sender configuration
sender:
  target: "log_file"        # Where to send metrics: "api" or "log_file"
  send_interval: 30s        # How often to send collected metrics

# API configuration (required if sender.target is "api")
api:
  url: "https://api.example.com/metrics"              # API endpoint
  project_id: "00000000-0000-0000-0000-000000000000"  # Project ID (UUID) to identify your project
  application_token: "your-application-token"         # Application token for authentication

# Log file configuration (used if sender.target is "log_file")
log_file:
  path: "logs/metrics.log"  # Path where metrics will be logged

# Application logging configuration
logging:
  file_path: "logs/monitorly.log"  # Path for application logs
```

### Machine Identification

The `machine_name` setting allows you to specify a custom identifier for the server:

- When sending to API: Machine name is included once at the top level of the request
- When logging to file: Machine name is not included (assumed to be local to the machine)
- If not specified, the system hostname will be used automatically
- This helps distinguish metrics from different servers in a central monitoring system

### API Authentication

When sending metrics to the API, the following fields are used:

- `project_id`: UUID that identifies your project within the Monitorly system (included in the URL)
- `application_token`: Authentication token used as Bearer token in API requests

### Metric Collection

Each metric type (CPU, RAM, Disk) can be independently configured with:

- `enabled`: Turn collection on/off
- `interval`: How frequently to collect the metric

### Disk Monitoring

For disk monitoring, you can specify multiple mount points to monitor:

- `path`: The filesystem path to monitor
- `label`: A user-friendly label for the mount point
- `collect_usage`: Whether to collect actual usage in bytes
- `collect_percent`: Whether to collect usage as a percentage

### Metric Delivery

The `sender` section configures how metrics are delivered:

- `target`: Either "api" to send to a central API or "log_file" to write to a local file
- `send_interval`: How frequently to send the collected metrics

## Metric Format

### Local File Format

When writing to a local file, metrics are stored as individual JSON objects:

```json
{
  "timestamp": "2023-04-01T12:34:56Z",
  "category": "system",
  "name": "cpu",
  "value": 45.67
}

{
  "timestamp": "2023-04-01T12:34:56Z",
  "category": "system",
  "name": "disk",
  "metadata": {
    "mountpoint": "/",
    "label": "root"
  },
  "value": {
    "percent": 76.54,
    "used": 120394752000,
    "total": 512000000000,
    "available": 391605248000
  }
}
```

### API Format

When sending to the API, metrics are sent as a single payload with the machine name at the top level:

```json
{
  "machine_name": "web-server-01",
  "metrics": [
    {
      "timestamp": "2023-04-01T12:34:56Z",
      "category": "system",
      "name": "cpu",
      "value": 45.67
    },
    {
      "timestamp": "2023-04-01T12:34:56Z",
      "category": "system",
      "name": "disk",
      "metadata": {
        "mountpoint": "/",
        "label": "root"
      },
      "value": {
        "percent": 76.54,
        "used": 120394752000,
        "total": 512000000000,
        "available": 391605248000
      }
    }
  ]
}
```