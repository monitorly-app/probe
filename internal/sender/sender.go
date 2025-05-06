package sender

import (
	"context"

	"github.com/monitorly-app/probe/internal/collector"
)

// Sender defines the interface for sending metrics
type Sender interface {
	// Send sends metrics using a background context
	Send(metrics []collector.Metrics) error

	// SendWithContext sends metrics with the provided context
	SendWithContext(ctx context.Context, metrics []collector.Metrics) error
}
