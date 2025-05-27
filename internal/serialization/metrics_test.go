package serialization

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/monitorly-app/probe/internal/collector"
)

func TestSerializeMetrics(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		metrics []collector.Metrics
		wantErr bool
	}{
		{
			name: "single metric",
			metrics: []collector.Metrics{
				{
					Timestamp: now,
					Category:  collector.CategorySystem,
					Name:      collector.NameCPU,
					Value:     75.5,
				},
			},
			wantErr: false,
		},
		{
			name: "multiple metrics",
			metrics: []collector.Metrics{
				{
					Timestamp: now,
					Category:  collector.CategorySystem,
					Name:      collector.NameCPU,
					Value:     75.5,
				},
				{
					Timestamp: now,
					Category:  collector.CategorySystem,
					Name:      collector.NameRAM,
					Value:     60.2,
				},
			},
			wantErr: false,
		},
		{
			name:    "empty metrics slice",
			metrics: []collector.Metrics{},
			wantErr: false,
		},
		{
			name:    "nil metrics slice",
			metrics: nil,
			wantErr: false,
		},
		{
			name: "metric with metadata",
			metrics: []collector.Metrics{
				{
					Timestamp: now,
					Category:  collector.CategorySystem,
					Name:      collector.NameDisk,
					Metadata: collector.MetricMetadata{
						"mountpoint": "/",
						"label":      "root",
					},
					Value: map[string]interface{}{
						"percent": 45.2,
						"used":    1024000,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "metric with complex value",
			metrics: []collector.Metrics{
				{
					Timestamp: now,
					Category:  collector.CategorySystem,
					Name:      collector.NameDisk,
					Value: map[string]interface{}{
						"nested": map[string]interface{}{
							"deep": "value",
						},
						"array": []int{1, 2, 3},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := SerializeMetrics(tt.metrics)

			if (err != nil) != tt.wantErr {
				t.Errorf("SerializeMetrics() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify it's valid JSON
				var result []collector.Metrics
				if err := json.Unmarshal(data, &result); err != nil {
					t.Errorf("SerializeMetrics() produced invalid JSON: %v", err)
					return
				}

				// Verify the length matches
				if len(result) != len(tt.metrics) {
					t.Errorf("SerializeMetrics() result length = %d, want %d", len(result), len(tt.metrics))
				}
			}
		})
	}
}

func TestSerializeMetricsIndented(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		metrics []collector.Metrics
		wantErr bool
	}{
		{
			name: "single metric indented",
			metrics: []collector.Metrics{
				{
					Timestamp: now,
					Category:  collector.CategorySystem,
					Name:      collector.NameCPU,
					Value:     75.5,
				},
			},
			wantErr: false,
		},
		{
			name: "multiple metrics indented",
			metrics: []collector.Metrics{
				{
					Timestamp: now,
					Category:  collector.CategorySystem,
					Name:      collector.NameCPU,
					Value:     75.5,
				},
				{
					Timestamp: now,
					Category:  collector.CategorySystem,
					Name:      collector.NameRAM,
					Value:     60.2,
				},
			},
			wantErr: false,
		},
		{
			name:    "empty metrics slice indented",
			metrics: []collector.Metrics{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := SerializeMetricsIndented(tt.metrics)

			if (err != nil) != tt.wantErr {
				t.Errorf("SerializeMetricsIndented() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify it's valid JSON
				var result []collector.Metrics
				if err := json.Unmarshal(data, &result); err != nil {
					t.Errorf("SerializeMetricsIndented() produced invalid JSON: %v", err)
					return
				}

				// Verify it contains indentation
				dataStr := string(data)
				if len(tt.metrics) > 0 && !strings.Contains(dataStr, "  ") {
					t.Errorf("SerializeMetricsIndented() output doesn't appear to be indented")
				}

				// Verify the length matches
				if len(result) != len(tt.metrics) {
					t.Errorf("SerializeMetricsIndented() result length = %d, want %d", len(result), len(tt.metrics))
				}
			}
		})
	}
}

func TestSerializeMetricIndented(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		metric  collector.Metrics
		wantErr bool
	}{
		{
			name: "simple metric",
			metric: collector.Metrics{
				Timestamp: now,
				Category:  collector.CategorySystem,
				Name:      collector.NameCPU,
				Value:     75.5,
			},
			wantErr: false,
		},
		{
			name: "metric with metadata",
			metric: collector.Metrics{
				Timestamp: now,
				Category:  collector.CategorySystem,
				Name:      collector.NameDisk,
				Metadata: collector.MetricMetadata{
					"mountpoint": "/",
					"label":      "root",
				},
				Value: map[string]interface{}{
					"percent": 45.2,
					"used":    1024000,
				},
			},
			wantErr: false,
		},
		{
			name: "metric with nil metadata",
			metric: collector.Metrics{
				Timestamp: now,
				Category:  collector.CategorySystem,
				Name:      collector.NameRAM,
				Metadata:  nil,
				Value:     60.2,
			},
			wantErr: false,
		},
		{
			name: "metric with complex value",
			metric: collector.Metrics{
				Timestamp: now,
				Category:  collector.CategorySystem,
				Name:      collector.NameDisk,
				Value: map[string]interface{}{
					"nested": map[string]interface{}{
						"deep": "value",
					},
					"array": []int{1, 2, 3},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := SerializeMetricIndented(tt.metric)

			if (err != nil) != tt.wantErr {
				t.Errorf("SerializeMetricIndented() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify it's valid JSON
				var result collector.Metrics
				if err := json.Unmarshal(data, &result); err != nil {
					t.Errorf("SerializeMetricIndented() produced invalid JSON: %v", err)
					return
				}

				// Verify it contains indentation
				dataStr := string(data)
				if !strings.Contains(dataStr, "  ") {
					t.Errorf("SerializeMetricIndented() output doesn't appear to be indented")
				}

				// Verify the category and name match
				if result.Category != tt.metric.Category {
					t.Errorf("SerializeMetricIndented() category = %v, want %v", result.Category, tt.metric.Category)
				}
				if result.Name != tt.metric.Name {
					t.Errorf("SerializeMetricIndented() name = %v, want %v", result.Name, tt.metric.Name)
				}
			}
		})
	}
}

func TestWriteMetricsTo(t *testing.T) {
	now := time.Now()

	testMetrics := []collector.Metrics{
		{
			Timestamp: now,
			Category:  collector.CategorySystem,
			Name:      collector.NameCPU,
			Value:     75.5,
		},
		{
			Timestamp: now,
			Category:  collector.CategorySystem,
			Name:      collector.NameRAM,
			Value:     60.2,
		},
	}

	tests := []struct {
		name    string
		metrics []collector.Metrics
		indent  bool
		wantErr bool
	}{
		{
			name:    "write metrics without indent",
			metrics: testMetrics,
			indent:  false,
			wantErr: false,
		},
		{
			name:    "write metrics with indent",
			metrics: testMetrics,
			indent:  true,
			wantErr: false,
		},
		{
			name:    "write empty metrics without indent",
			metrics: []collector.Metrics{},
			indent:  false,
			wantErr: false,
		},
		{
			name:    "write empty metrics with indent",
			metrics: []collector.Metrics{},
			indent:  true,
			wantErr: false,
		},
		{
			name:    "write single metric with indent",
			metrics: testMetrics[:1],
			indent:  true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := WriteMetricsTo(&buf, tt.metrics, tt.indent)

			if (err != nil) != tt.wantErr {
				t.Errorf("WriteMetricsTo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				output := buf.String()

				// For empty metrics, we expect a newline-terminated empty array
				if len(tt.metrics) == 0 {
					if output != "[]\n" {
						t.Errorf("WriteMetricsTo() empty metrics output = %q, want %q", output, "[]\n")
					}
					return
				}

				// For non-empty metrics, verify output ends with newline
				if !strings.HasSuffix(output, "\n") {
					t.Errorf("WriteMetricsTo() output doesn't end with newline")
				}

				if tt.indent && len(tt.metrics) > 0 {
					// For indented output, each metric should be on separate lines
					lines := strings.Split(strings.TrimSpace(output), "\n")

					// Should have content for each metric plus separators
					if len(lines) < len(tt.metrics) {
						t.Errorf("WriteMetricsTo() indented output has %d lines, want at least %d", len(lines), len(tt.metrics))
					}

					// Verify indentation is present
					if !strings.Contains(output, "  ") {
						t.Errorf("WriteMetricsTo() indented output doesn't contain indentation")
					}
				} else if !tt.indent && len(tt.metrics) > 0 {
					// For non-indented output, should be valid JSON array
					trimmed := strings.TrimSpace(output)
					var result []collector.Metrics
					if err := json.Unmarshal([]byte(trimmed), &result); err != nil {
						t.Errorf("WriteMetricsTo() non-indented output is not valid JSON: %v", err)
					}
				}
			}
		})
	}
}

func TestWriteMetricsToWithFailingWriter(t *testing.T) {
	// Create a writer that always fails
	failingWriter := &failingWriter{}

	metrics := []collector.Metrics{
		{
			Timestamp: time.Now(),
			Category:  collector.CategorySystem,
			Name:      collector.NameCPU,
			Value:     75.5,
		},
	}

	// Test with indent=false
	err := WriteMetricsTo(failingWriter, metrics, false)
	if err == nil {
		t.Errorf("WriteMetricsTo() with failing writer expected error but got none")
	}

	// Test with indent=true
	err = WriteMetricsTo(failingWriter, metrics, true)
	if err == nil {
		t.Errorf("WriteMetricsTo() with failing writer and indent expected error but got none")
	}
}

// failingWriter is a writer that always returns an error
type failingWriter struct{}

func (fw *failingWriter) Write(p []byte) (n int, err error) {
	return 0, bytes.ErrTooLarge
}

func TestSerializationRoundTrip(t *testing.T) {
	now := time.Now()

	originalMetrics := []collector.Metrics{
		{
			Timestamp: now,
			Category:  collector.CategorySystem,
			Name:      collector.NameCPU,
			Value:     75.5,
		},
		{
			Timestamp: now,
			Category:  collector.CategorySystem,
			Name:      collector.NameDisk,
			Metadata: collector.MetricMetadata{
				"mountpoint": "/",
				"label":      "root",
			},
			Value: map[string]interface{}{
				"percent": 45.2,
				"used":    1024000,
			},
		},
	}

	// Test round trip with SerializeMetrics
	data, err := SerializeMetrics(originalMetrics)
	if err != nil {
		t.Fatalf("SerializeMetrics() failed: %v", err)
	}

	var deserializedMetrics []collector.Metrics
	if err := json.Unmarshal(data, &deserializedMetrics); err != nil {
		t.Fatalf("json.Unmarshal() failed: %v", err)
	}

	if len(deserializedMetrics) != len(originalMetrics) {
		t.Errorf("Round trip length mismatch: got %d, want %d", len(deserializedMetrics), len(originalMetrics))
	}

	// Test round trip with SerializeMetricsIndented
	indentedData, err := SerializeMetricsIndented(originalMetrics)
	if err != nil {
		t.Fatalf("SerializeMetricsIndented() failed: %v", err)
	}

	var deserializedIndentedMetrics []collector.Metrics
	if err := json.Unmarshal(indentedData, &deserializedIndentedMetrics); err != nil {
		t.Fatalf("json.Unmarshal() of indented data failed: %v", err)
	}

	if len(deserializedIndentedMetrics) != len(originalMetrics) {
		t.Errorf("Indented round trip length mismatch: got %d, want %d", len(deserializedIndentedMetrics), len(originalMetrics))
	}
}

// BenchmarkSerializeMetrics benchmarks the SerializeMetrics function
func BenchmarkSerializeMetrics(b *testing.B) {
	now := time.Now()
	metrics := []collector.Metrics{
		{
			Timestamp: now,
			Category:  collector.CategorySystem,
			Name:      collector.NameCPU,
			Value:     75.5,
		},
		{
			Timestamp: now,
			Category:  collector.CategorySystem,
			Name:      collector.NameRAM,
			Value:     60.2,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := SerializeMetrics(metrics)
		if err != nil {
			b.Fatalf("SerializeMetrics() failed: %v", err)
		}
	}
}

// BenchmarkSerializeMetricsIndented benchmarks the SerializeMetricsIndented function
func BenchmarkSerializeMetricsIndented(b *testing.B) {
	now := time.Now()
	metrics := []collector.Metrics{
		{
			Timestamp: now,
			Category:  collector.CategorySystem,
			Name:      collector.NameCPU,
			Value:     75.5,
		},
		{
			Timestamp: now,
			Category:  collector.CategorySystem,
			Name:      collector.NameRAM,
			Value:     60.2,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := SerializeMetricsIndented(metrics)
		if err != nil {
			b.Fatalf("SerializeMetricsIndented() failed: %v", err)
		}
	}
}

// BenchmarkWriteMetricsTo benchmarks the WriteMetricsTo function
func BenchmarkWriteMetricsTo(b *testing.B) {
	now := time.Now()
	metrics := []collector.Metrics{
		{
			Timestamp: now,
			Category:  collector.CategorySystem,
			Name:      collector.NameCPU,
			Value:     75.5,
		},
		{
			Timestamp: now,
			Category:  collector.CategorySystem,
			Name:      collector.NameRAM,
			Value:     60.2,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		err := WriteMetricsTo(&buf, metrics, false)
		if err != nil {
			b.Fatalf("WriteMetricsTo() failed: %v", err)
		}
	}
}
