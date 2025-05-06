# Server Probes (Go)

The Go-based probes, configured via a local file on each monitored server, will be responsible for collecting data and sending it to the central server. Here are potential features:

* **Configurable Monitoring Targets:**
    * **Disk Space Monitoring:**
        * Specify mount points to monitor.
        * Set warning and critical threshold percentages for disk usage.
        * Optionally monitor specific directories for size changes (useful for log growth).
    * **Network Connectivity Monitoring:**
        * Ping specific hostnames or IP addresses.
        * Check for successful connection to specific TCP/UDP ports on remote hosts.
        * Measure latency and packet loss (if applicable).
    * **Service Status Monitoring:**
        * Specify service names (e.g., `nginx`, `php-fpm`, `mysql`, `postgresql`).
        * Check if the service is in an active/running state.
        * Optionally attempt to restart the service (with proper permissions and configuration) and report success/failure.
    * **Port Monitoring:**
        * Check if specific TCP or UDP ports are listening on the local server.
    * **Crontab Execution Monitoring:**
        * Monitor the execution status (success/failure) and output of specific cron jobs. This could involve parsing cron logs or using a custom wrapper script.
        * Optionally alert if a scheduled job doesn't run within an expected timeframe.
    * **CPU Usage Monitoring:**
        * Track overall CPU utilization.
        * Monitor CPU usage per core.
        * Set warning and critical thresholds for CPU usage.
    * **Memory Usage Monitoring:**
        * Track total and available RAM.
        * Monitor swap usage.
        * Set warning and critical thresholds for memory and swap usage.
    * **Load Average Monitoring:**
        * Track the system load average (1, 5, and 15-minute averages).
        * Set warning and critical thresholds for load.
    * **Process Monitoring:**
        * Check if specific processes are running (by name or PID).
        * Monitor CPU and memory consumption of specific processes.
        * Optionally alert if a critical process terminates unexpectedly.
    * **Log File Monitoring (Basic):**
        * Monitor specific log files for new entries matching defined patterns (simple regex).
        * Report the occurrence of specific error or warning messages.
    * **Custom Script Execution:**
        * Allow users to define and execute custom scripts (in any language) and report their exit code and output to the central server. This provides high extensibility.
    * **Network Interface Monitoring:**
        * Track network traffic (inbound/outbound bytes/packets) on specific interfaces.
        * Monitor interface status (up/down).
        * Set thresholds for bandwidth usage.

* **Configuration Management:**
    * Simple and well-documented configuration file format (e.g., YAML or TOML).
    * Ability to specify monitoring intervals for each check.
    * Secure handling of sensitive information (if any).
    * Mechanism for the central server to potentially push configuration updates to the probes (advanced feature).

* **Communication with Central Server:**
    * Secure and efficient communication protocol (e.g., gRPC or lightweight HTTP/JSON with authentication).
    * Reliable delivery of monitoring data, potentially with buffering or retry mechanisms in case of network issues.
    * Clear identification of the sending server.

* **Resource Efficiency:**
    * Designed to have a minimal impact on the monitored server's resources (CPU, memory, network).

* **Self-Monitoring:**
    * Basic self-health checks (e.g., reporting its own CPU/memory usage, uptime, and connection status to the central server).