# Monitorly Probe

A lightweight server monitoring probe that collects system metrics and sends them to a central API or logs them to a file.

## Features

- Independent metric collection for CPU, RAM, and Disk usage
- Configurable collection intervals for each metric type
- Flexible disk monitoring with support for multiple mount points
- Machine identification for multi-server monitoring
- Metrics can be sent to a central API or logged to a local file
- Low resource footprint

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
./bin/monitorly-probe
```

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
  url: "https://api.example.com/metrics"  # API endpoint
  key: "your-api-key"                     # API authentication key

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

## License

This project is licensed under the MIT License - see the LICENSE file for details.