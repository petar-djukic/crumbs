package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr error
	}{
		{
			name:    "valid sqlite config",
			config:  Config{Backend: "sqlite", DataDir: "/tmp/data"},
			wantErr: nil,
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
			name:    "empty DataDir for sqlite",
			config:  Config{Backend: "sqlite", DataDir: ""},
			wantErr: ErrDataDirEmpty,
		},
		{
			name: "valid config with SQLiteConfig",
			config: Config{
				Backend: "sqlite",
				DataDir: "/tmp/data",
				SQLiteConfig: &SQLiteConfig{
					SyncStrategy:  "immediate",
					BatchSize:     100,
					BatchInterval: 5,
				},
			},
			wantErr: nil,
		},
		{
			name: "valid config with empty sync strategy uses default",
			config: Config{
				Backend: "sqlite",
				DataDir: "/tmp/data",
				SQLiteConfig: &SQLiteConfig{
					SyncStrategy: "",
				},
			},
			wantErr: nil,
		},
		{
			name: "valid config with on_close sync strategy",
			config: Config{
				Backend: "sqlite",
				DataDir: "/tmp/data",
				SQLiteConfig: &SQLiteConfig{
					SyncStrategy: "on_close",
				},
			},
			wantErr: nil,
		},
		{
			name: "valid config with batch sync strategy",
			config: Config{
				Backend: "sqlite",
				DataDir: "/tmp/data",
				SQLiteConfig: &SQLiteConfig{
					SyncStrategy: "batch",
				},
			},
			wantErr: nil,
		},
		{
			name: "invalid sync strategy",
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
			name: "negative batch size",
			config: Config{
				Backend: "sqlite",
				DataDir: "/tmp/data",
				SQLiteConfig: &SQLiteConfig{
					BatchSize: -1,
				},
			},
			wantErr: ErrBatchSizeInvalid,
		},
		{
			name: "negative batch interval",
			config: Config{
				Backend: "sqlite",
				DataDir: "/tmp/data",
				SQLiteConfig: &SQLiteConfig{
					BatchInterval: -1,
				},
			},
			wantErr: ErrBatchIntervalInvalid,
		},
		{
			name: "zero batch size and interval are valid",
			config: Config{
				Backend: "sqlite",
				DataDir: "/tmp/data",
				SQLiteConfig: &SQLiteConfig{
					BatchSize:     0,
					BatchInterval: 0,
				},
			},
			wantErr: nil,
		},
		{
			name:    "nil SQLiteConfig is valid",
			config:  Config{Backend: "sqlite", DataDir: "/tmp/data"},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSQLiteConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  SQLiteConfig
		wantErr error
	}{
		{
			name:    "zero value is valid",
			config:  SQLiteConfig{},
			wantErr: nil,
		},
		{
			name:    "all valid strategies",
			config:  SQLiteConfig{SyncStrategy: "immediate"},
			wantErr: nil,
		},
		{
			name:    "unknown strategy",
			config:  SQLiteConfig{SyncStrategy: "never"},
			wantErr: ErrSyncStrategyUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
