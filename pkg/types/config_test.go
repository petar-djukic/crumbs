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
			name:    "empty backend returns ErrBackendEmpty",
			config:  Config{Backend: "", DataDir: "/tmp/data"},
			wantErr: ErrBackendEmpty,
		},
		{
			name:    "unknown backend returns ErrBackendUnknown",
			config:  Config{Backend: "postgres", DataDir: "/tmp/data"},
			wantErr: ErrBackendUnknown,
		},
		{
			name:    "valid sqlite config",
			config:  Config{Backend: "sqlite", DataDir: "/tmp/data"},
			wantErr: nil,
		},
		{
			name:    "sqlite with empty DataDir is valid at config level",
			config:  Config{Backend: "sqlite", DataDir: ""},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
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
