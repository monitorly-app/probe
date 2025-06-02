package system

import (
	"fmt"
	"time"

	"github.com/monitorly-app/probe/internal/collector"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

// PortCollector implements the collector.Collector interface for port monitoring metrics
type PortCollector struct{}

// NewPortCollector creates a new instance of PortCollector
func NewPortCollector() collector.Collector {
	return &PortCollector{}
}

// PortInfo represents information about an open port and its process
type PortInfo struct {
	Protocol    string `json:"protocol"`     // TCP or UDP
	LocalAddr   string `json:"local_addr"`   // Local IP address
	LocalPort   uint32 `json:"local_port"`   // Local port number
	RemoteAddr  string `json:"remote_addr"`  // Remote IP address (for established connections)
	RemotePort  uint32 `json:"remote_port"`  // Remote port number (for established connections)
	Status      string `json:"status"`       // Connection status (LISTEN, ESTABLISHED, etc.)
	ProcessID   int32  `json:"process_id"`   // Process ID (PID) listening on the port
	ProcessName string `json:"process_name"` // Process name
}

// Collect gathers port monitoring metrics by listing all open TCP/UDP ports
func (c *PortCollector) Collect() ([]collector.Metrics, error) {
	metrics := make([]collector.Metrics, 0, 1)
	now := time.Now()

	ports, err := c.getOpenPorts()
	if err != nil {
		return metrics, fmt.Errorf("failed to get open ports: %w", err)
	}

	// Create a single metric with all open ports
	metrics = append(metrics, collector.Metrics{
		Timestamp: now,
		Category:  collector.CategorySystem,
		Name:      collector.NamePort,
		Value:     ports,
	})

	return metrics, nil
}

// getOpenPorts retrieves all open TCP and UDP ports with their associated processes
func (c *PortCollector) getOpenPorts() ([]PortInfo, error) {
	var allPorts []PortInfo

	// Get TCP connections
	tcpConns, err := net.Connections("tcp")
	if err != nil {
		return nil, fmt.Errorf("failed to get TCP connections: %w", err)
	}

	for _, conn := range tcpConns {
		portInfo := PortInfo{
			Protocol:   "tcp",
			LocalAddr:  conn.Laddr.IP,
			LocalPort:  conn.Laddr.Port,
			RemoteAddr: conn.Raddr.IP,
			RemotePort: conn.Raddr.Port,
			Status:     conn.Status,
			ProcessID:  conn.Pid,
		}

		// Get process name if PID is available
		if conn.Pid > 0 {
			if processName, err := c.getProcessName(conn.Pid); err == nil {
				portInfo.ProcessName = processName
			}
		}

		allPorts = append(allPorts, portInfo)
	}

	// Get UDP connections
	udpConns, err := net.Connections("udp")
	if err != nil {
		return nil, fmt.Errorf("failed to get UDP connections: %w", err)
	}

	for _, conn := range udpConns {
		portInfo := PortInfo{
			Protocol:   "udp",
			LocalAddr:  conn.Laddr.IP,
			LocalPort:  conn.Laddr.Port,
			RemoteAddr: conn.Raddr.IP,
			RemotePort: conn.Raddr.Port,
			Status:     conn.Status,
			ProcessID:  conn.Pid,
		}

		// Get process name if PID is available
		if conn.Pid > 0 {
			if processName, err := c.getProcessName(conn.Pid); err == nil {
				portInfo.ProcessName = processName
			}
		}

		allPorts = append(allPorts, portInfo)
	}

	return allPorts, nil
}

// getProcessName retrieves the process name for a given PID
func (c *PortCollector) getProcessName(pid int32) (string, error) {
	proc, err := process.NewProcess(pid)
	if err != nil {
		return "", fmt.Errorf("failed to get process for PID %d: %w", pid, err)
	}

	name, err := proc.Name()
	if err != nil {
		return "", fmt.Errorf("failed to get process name for PID %d: %w", pid, err)
	}

	return name, nil
}
