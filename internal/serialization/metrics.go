package serialization

import (
	"encoding/json"
	"fmt"
	"io"
	"log"

	"github.com/monitorly-app/probe/internal/collector"
)

// SerializeMetrics converts a slice of metrics to a JSON byte array
func SerializeMetrics(metrics []collector.Metrics) ([]byte, error) {
	return json.Marshal(metrics)
}

// SerializeMetricsIndented converts a slice of metrics to a pretty-printed JSON byte array
func SerializeMetricsIndented(metrics []collector.Metrics) ([]byte, error) {
	return json.MarshalIndent(metrics, "", "  ")
}

// SerializeMetricIndented converts a single metric to a pretty-printed JSON byte array
func SerializeMetricIndented(metric collector.Metrics) ([]byte, error) {
	jsonData, err := json.MarshalIndent(metric, "", "  ")
	if err != nil {
		log.Printf("ERROR: Failed to marshal metric %s: %v", metric.Name, err)
	}

	return jsonData, err
}

// WriteMetricsTo writes serialized metrics to the provided writer
func WriteMetricsTo(w io.Writer, metrics []collector.Metrics, indent bool) error {
	// For empty metrics, always write an empty array with a newline
	if len(metrics) == 0 {
		if _, err := io.WriteString(w, "[]\n"); err != nil {
			return fmt.Errorf("failed to write empty array: %w", err)
		}
		return nil
	}

	if indent {
		for i, metric := range metrics {
			// Write each metric separately as indented JSON
			jsonData, err := SerializeMetricIndented(metric)
			if err != nil {
				return fmt.Errorf("failed to marshal metric: %w", err)
			}

			if _, err := w.Write(jsonData); err != nil {
				log.Printf("ERROR: Failed to write metric %s to output: %v", metric.Name, err)
				return fmt.Errorf("failed to write to output: %w", err)
			}

			// Add newline after each metric
			if _, err := io.WriteString(w, "\n"); err != nil {
				return fmt.Errorf("failed to write newline: %w", err)
			}

			// Add a blank line between metrics except after the last one
			if i < len(metrics)-1 {
				if _, err := io.WriteString(w, "\n"); err != nil {
					return fmt.Errorf("failed to write separator: %w", err)
				}
			}
		}
	} else {
		// Write all metrics as a single JSON array
		jsonData, err := SerializeMetrics(metrics)
		if err != nil {
			return fmt.Errorf("failed to marshal metrics: %w", err)
		}

		if _, err := w.Write(jsonData); err != nil {
			return fmt.Errorf("failed to write to output: %w", err)
		}

		// Add newline after the JSON array
		if _, err := io.WriteString(w, "\n"); err != nil {
			return fmt.Errorf("failed to write newline: %w", err)
		}
	}

	return nil
}
