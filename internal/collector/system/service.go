package system

import (
	"os/exec"
	"time"

	"github.com/monitorly-app/probe/internal/collector"
	"github.com/monitorly-app/probe/internal/config"
)

// ServiceCollector implements the collector.Collector interface for service metrics
type ServiceCollector struct {
	Services []config.Service
}

// NewServiceCollector creates a new instance of ServiceCollector
func NewServiceCollector(services []config.Service) collector.Collector {
	return &ServiceCollector{
		Services: services,
	}
}

// checkServiceStatusSystemd checks service status using systemd
func (c *ServiceCollector) checkServiceStatusSystemd(serviceName string) bool {
	cmd := exec.Command("systemctl", "is-active", "--quiet", serviceName)
	return cmd.Run() == nil
}

// checkServiceStatusSysV checks service status using SysV init scripts
func (c *ServiceCollector) checkServiceStatusSysV(serviceName string) bool {
	// Try service command first (more portable)
	cmd := exec.Command("service", serviceName, "status")
	if err := cmd.Run(); err == nil {
		return true
	}

	// Fallback to direct init.d script check
	cmd = exec.Command("/etc/init.d/"+serviceName, "status")
	return cmd.Run() == nil
}

// Collect gathers service status metrics
func (c *ServiceCollector) Collect() ([]collector.Metrics, error) {
	metrics := make([]collector.Metrics, 0, len(c.Services))
	now := time.Now()

	for _, service := range c.Services {
		// Try systemd first, then fall back to SysV if systemd check fails
		var isActive bool

		// Check if systemctl exists
		if _, err := exec.LookPath("systemctl"); err == nil {
			isActive = c.checkServiceStatusSystemd(service.Name)
		} else {
			// Fallback to SysV init
			isActive = c.checkServiceStatusSysV(service.Name)
		}

		// Convert status to float (0.0 = active, 1.0 = inactive)
		status := 1.0
		if isActive {
			status = 0.0
		}

		metrics = append(metrics, collector.Metrics{
			Timestamp: now,
			Category:  collector.CategorySystem,
			Name:      collector.NameService,
			Metadata: collector.MetricMetadata{
				"name":  service.Name,
				"label": service.Label,
			},
			Value: status,
		})
	}

	return metrics, nil
}
