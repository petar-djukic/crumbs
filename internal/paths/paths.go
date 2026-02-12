// Package paths resolves configuration and data directory locations.
// Implements: prd010-configuration-directories (R1.2, R1.3, R2.2, R2.3, R8);
//
//	rel01.1-uc003-configuration-loading (F1-F5, S1-S7).
package paths

import (
	"os"
	"path/filepath"
	"runtime"
)

// CWD-relative directory names per prd010 R1.2 and R2.2.
const (
	DefaultConfigDirName = ".crumbs"
	DefaultDataDirName   = ".crumbs-db"
)

// Environment variable names for directory overrides.
const (
	EnvConfigDir = "CRUMBS_CONFIG_DIR"
	EnvDataDir   = "CRUMBS_DATA_DIR"
)

// platformDir holds platform-detection functions that can be overridden in tests.
var platformDir = struct {
	homeDir       func() (string, error)
	userConfigDir func() (string, error)
}{
	homeDir:       os.UserHomeDir,
	userConfigDir: os.UserConfigDir,
}

// DefaultConfigDir returns the platform-specific default configuration directory.
//
// Linux:   $XDG_CONFIG_HOME/crumbs (fallback ~/.config/crumbs)
// macOS:   ~/Library/Application Support/crumbs
// Windows: %APPDATA%/crumbs
func DefaultConfigDir() (string, error) {
	switch runtime.GOOS {
	case "linux":
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, "crumbs"), nil
		}
		home, err := platformDir.homeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".config", "crumbs"), nil
	default:
		// macOS and Windows use os.UserConfigDir which returns
		// ~/Library/Application Support on macOS and %APPDATA% on Windows.
		dir, err := platformDir.userConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "crumbs"), nil
	}
}

// DefaultDataDir returns the platform-specific default data directory.
//
// Linux:   $XDG_DATA_HOME/crumbs (fallback ~/.local/share/crumbs)
// macOS:   ~/Library/Application Support/crumbs
// Windows: %APPDATA%/crumbs
func DefaultDataDir() (string, error) {
	switch runtime.GOOS {
	case "linux":
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, "crumbs"), nil
		}
		home, err := platformDir.homeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "share", "crumbs"), nil
	default:
		// macOS and Windows: same as config dir.
		dir, err := platformDir.userConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "crumbs"), nil
	}
}

// ResolveConfigDir returns the configuration directory following the precedence
// chain: flag > CRUMBS_CONFIG_DIR env > DefaultConfigDir().
//
// If flag is non-empty it wins. Otherwise the CRUMBS_CONFIG_DIR environment
// variable is checked. If neither is set, the platform default is returned.
func ResolveConfigDir(flag string) (string, error) {
	if flag != "" {
		return filepath.Abs(flag)
	}
	if env := os.Getenv(EnvConfigDir); env != "" {
		return filepath.Abs(env)
	}
	return DefaultConfigDir()
}

// ResolveDataDir returns the data directory following the precedence chain:
// flag > configYAMLValue > CRUMBS_DATA_DIR env > DefaultDataDir().
//
// The CWD-relative default ($(CWD)/.crumbs-db) is preserved as the primary
// mode when no override is active, matching existing behavior per the task
// design decisions.
func ResolveDataDir(flag, configYAMLValue string) (string, error) {
	if flag != "" {
		return filepath.Abs(flag)
	}
	if configYAMLValue != "" {
		return filepath.Abs(configYAMLValue)
	}
	if env := os.Getenv(EnvDataDir); env != "" {
		return filepath.Abs(env)
	}
	// CWD-relative default preserves current behavior.
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, DefaultDataDirName), nil
}
