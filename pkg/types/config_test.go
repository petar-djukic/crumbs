package types

import (
	"errors"
	"testing"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr error
	}{
		{
			name:   "valid sqlite config",
			config: Config{Backend: "sqlite", DataDir: "/tmp/data"},
		},
		{
			name:    "empty backend",
			config:  Config{Backend: "", DataDir: "/tmp/data"},
			wantErr: ErrBackendEmpty,
		},
		{
			name:    "unknown backend",
			config:  Config{Backend: "postgres", DataDir: "/tmp/data"},
			wantErr: ErrBackendUnknown,
		},
		{
			name:    "empty data dir",
			config:  Config{Backend: "sqlite", DataDir: ""},
			wantErr: ErrDataDirEmpty,
		},
		{
			name: "valid sqlite config with sync strategy",
			config: Config{
				Backend: "sqlite",
				DataDir: "/tmp/data",
				SQLiteConfig: &SQLiteConfig{
					SyncStrategy: "immediate",
				},
			},
		},
		{
			name: "unknown sync strategy",
			config: Config{
				Backend: "sqlite",
				DataDir: "/tmp/data",
				SQLiteConfig: &SQLiteConfig{
					SyncStrategy: "unknown",
				},
			},
			wantErr: ErrSyncStrategyUnknown,
		},
		{
			name: "batch with invalid batch size",
			config: Config{
				Backend: "sqlite",
				DataDir: "/tmp/data",
				SQLiteConfig: &SQLiteConfig{
					SyncStrategy:  "batch",
					BatchSize:     0,
					BatchInterval: 100,
				},
			},
			wantErr: ErrBatchSizeInvalid,
		},
		{
			name: "batch with invalid batch interval",
			config: Config{
				Backend: "sqlite",
				DataDir: "/tmp/data",
				SQLiteConfig: &SQLiteConfig{
					SyncStrategy:  "batch",
					BatchSize:     10,
					BatchInterval: 0,
				},
			},
			wantErr: ErrBatchIntervalInvalid,
		},
		{
			name: "valid batch config",
			config: Config{
				Backend: "sqlite",
				DataDir: "/tmp/data",
				SQLiteConfig: &SQLiteConfig{
					SyncStrategy:  "batch",
					BatchSize:     10,
					BatchInterval: 100,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error %v, got nil", tt.wantErr)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}
