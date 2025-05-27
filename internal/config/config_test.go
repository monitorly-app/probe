package config

import (
	"testing"
	"time"
)

func TestGetUpdateCheckTime(t *testing.T) {
	tests := []struct {
		name       string
		checkTime  string
		wantHour   int
		wantMinute int
		wantErr    bool
	}{
		{
			name:       "Default midnight",
			checkTime:  "",
			wantHour:   0,
			wantMinute: 0,
			wantErr:    false,
		},
		{
			name:       "Custom time",
			checkTime:  "13:30",
			wantHour:   13,
			wantMinute: 30,
			wantErr:    false,
		},
		{
			name:      "Invalid format",
			checkTime: "25:00",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			cfg.Updates.CheckTime = tt.checkTime

			got, err := cfg.GetUpdateCheckTime()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetUpdateCheckTime() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			if got.Hour() != tt.wantHour {
				t.Errorf("GetUpdateCheckTime() hour = %v, want %v", got.Hour(), tt.wantHour)
			}
			if got.Minute() != tt.wantMinute {
				t.Errorf("GetUpdateCheckTime() minute = %v, want %v", got.Minute(), tt.wantMinute)
			}

			// Check that the returned time is in the future
			if !got.After(time.Now()) {
				t.Error("GetUpdateCheckTime() returned a time in the past")
			}
		})
	}
}

func TestGetUpdateRetryDelay(t *testing.T) {
	tests := []struct {
		name       string
		retryDelay time.Duration
		want       time.Duration
	}{
		{
			name:       "Default 1 hour",
			retryDelay: 0,
			want:       time.Hour,
		},
		{
			name:       "Custom delay",
			retryDelay: 30 * time.Minute,
			want:       30 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			cfg.Updates.RetryDelay = tt.retryDelay

			if got := cfg.GetUpdateRetryDelay(); got != tt.want {
				t.Errorf("GetUpdateRetryDelay() = %v, want %v", got, tt.want)
			}
		})
	}
}
