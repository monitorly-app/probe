package system

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/monitorly-app/probe/internal/collector"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
)

// SystemInfo represents detailed system information
type SystemInfo struct {
	Hostname      string     `json:"hostname"`
	PublicIP      string     `json:"public_ip"`
	OS            string     `json:"os"`
	OSVersion     string     `json:"os_version"`
	KernelVersion string     `json:"kernel_version"`
	CPU           CPUInfo    `json:"cpu"`
	RAM           RAMInfo    `json:"ram"`
	Disks         []DiskInfo `json:"disks"`
	Services      []string   `json:"services"`
	LastBootTime  int64      `json:"last_boot_time"`
}

// CPUInfo represents CPU information
type CPUInfo struct {
	Name      string  `json:"name"`
	Cores     int32   `json:"cores"`
	Frequency float64 `json:"frequency_mhz"`
}

// RAMInfo represents RAM information
type RAMInfo struct {
	Total uint64 `json:"total_bytes"`
}

// DiskInfo represents disk information
type DiskInfo struct {
	Mountpoint string `json:"mountpoint"`
	Label      string `json:"label"`
	Total      uint64 `json:"total_bytes"`
}

// SystemInfoCollector implements the collector.Collector interface for system information
type SystemInfoCollector struct{}

// NewSystemInfoCollector creates a new instance of SystemInfoCollector
func NewSystemInfoCollector() collector.Collector {
	return &SystemInfoCollector{}
}

// Collect gathers system information
func (c *SystemInfoCollector) Collect() ([]collector.Metrics, error) {
	metrics := make([]collector.Metrics, 0, 1)
	now := time.Now()

	info, err := c.getSystemInfo()
	if err != nil {
		return metrics, fmt.Errorf("failed to get system info: %w", err)
	}

	metrics = append(metrics, collector.Metrics{
		Timestamp: now,
		Category:  collector.CategorySystem,
		Name:      collector.NameSystemInfo,
		Value:     info,
	})

	return metrics, nil
}

// getSystemInfo collects all system information
func (c *SystemInfoCollector) getSystemInfo() (*SystemInfo, error) {
	info := &SystemInfo{}

	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %w", err)
	}
	info.Hostname = hostname

	// Get public IP
	publicIP, err := c.getPublicIP()
	if err != nil {
		// Log error but continue - public IP is not critical
		fmt.Printf("Warning: Failed to get public IP: %v\n", err)
	}
	info.PublicIP = publicIP

	// Get OS info
	hostInfo, err := host.Info()
	if err != nil {
		return nil, fmt.Errorf("failed to get host info: %w", err)
	}
	info.OS = runtime.GOOS
	info.OSVersion = hostInfo.PlatformVersion
	info.KernelVersion = hostInfo.KernelVersion
	info.LastBootTime = int64(hostInfo.BootTime)

	// Get CPU info
	cpuInfo, err := cpu.Info()
	if err != nil {
		return nil, fmt.Errorf("failed to get CPU info: %w", err)
	}
	if len(cpuInfo) > 0 {
		info.CPU = CPUInfo{
			Name:      cpuInfo[0].ModelName,
			Cores:     cpuInfo[0].Cores,
			Frequency: cpuInfo[0].Mhz,
		}
	}

	// Get RAM info
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("failed to get memory info: %w", err)
	}
	info.RAM = RAMInfo{
		Total: memInfo.Total,
	}

	// Get disk info
	partitions, err := disk.Partitions(false) // false = physical partitions only
	if err != nil {
		return nil, fmt.Errorf("failed to get disk partitions: %w", err)
	}

	for _, partition := range partitions {
		// Skip system and swap partitions
		if strings.HasPrefix(partition.Device, "/dev/loop") ||
			strings.HasPrefix(partition.Device, "/dev/ram") ||
			strings.Contains(partition.Mountpoint, "/boot") ||
			strings.Contains(partition.Mountpoint, "/snap") ||
			partition.Fstype == "squashfs" ||
			partition.Fstype == "swap" {
			continue
		}

		usage, err := disk.Usage(partition.Mountpoint)
		if err != nil {
			// Skip this partition if we can't get usage info
			continue
		}

		info.Disks = append(info.Disks, DiskInfo{
			Mountpoint: partition.Mountpoint,
			Label:      partition.Device,
			Total:      usage.Total,
		})
	}

	// Get system services using systemctl
	if _, err := exec.LookPath("systemctl"); err == nil {
		cmd := exec.Command("systemctl", "list-units", "--type=service", "--state=active", "--no-pager", "--no-legend")
		output, err := cmd.Output()
		if err != nil {
			// Log error but continue - service list is not critical
			fmt.Printf("Warning: Failed to get service list: %v\n", err)
		} else {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if line == "" {
					continue
				}
				// Extract service name from the first column
				fields := strings.Fields(line)
				if len(fields) > 0 {
					serviceName := strings.TrimSuffix(fields[0], ".service")
					info.Services = append(info.Services, serviceName)
				}
			}
		}
	} else {
		// Try SysV init if systemctl is not available
		cmd := exec.Command("service", "--status-all")
		output, err := cmd.Output()
		if err != nil {
			// Log error but continue - service list is not critical
			fmt.Printf("Warning: Failed to get service list: %v\n", err)
		} else {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if line == "" {
					continue
				}
				// Extract service name from the line
				fields := strings.Fields(line)
				if len(fields) > 1 {
					info.Services = append(info.Services, fields[len(fields)-1])
				}
			}
		}
	}

	return info, nil
}

// getPublicIP retrieves the public IP address using an external service
func (c *SystemInfoCollector) getPublicIP() (string, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get("https://api.ipify.org")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get public IP: status code %d", resp.StatusCode)
	}

	ip := make([]byte, 45) // Maximum IPv6 length
	n, err := resp.Body.Read(ip)
	if err != nil {
		return "", err
	}

	return string(ip[:n]), nil
}
