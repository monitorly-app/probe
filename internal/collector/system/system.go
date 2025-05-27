package system

import (
	"github.com/monitorly-app/probe/internal/collector"
	"github.com/monitorly-app/probe/internal/config"
)

// NewCollectors returns a map of system collector constructors
func NewCollectors() map[string]func() collector.Collector {
	return map[string]func() collector.Collector{
		"cpu": func() collector.Collector {
			return NewCPUCollector()
		},
		"ram": func() collector.Collector {
			return NewRAMCollector()
		},
	}
}

// NewDiskCollectorFunc returns a function that creates a new disk collector with the specified mount points
func NewDiskCollectorFunc(mountPoints []config.MountPoint) func() collector.Collector {
	return func() collector.Collector {
		return NewDiskCollector(mountPoints)
	}
}

// NewServiceCollectorFunc returns a function that creates a new service collector with the specified services
func NewServiceCollectorFunc(services []config.Service) func() collector.Collector {
	return func() collector.Collector {
		return NewServiceCollector(services)
	}
}
