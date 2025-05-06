package sender

import (
	"github.com/monitorly-app/probe/internal/collector"
)

// Sender defines the interface for sending metrics
type Sender interface {
	Send(metrics []collector.Metrics) error
}
