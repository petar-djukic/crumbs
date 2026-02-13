package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfigDir_Linux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only test")
	}

	t.Run("uses XDG_CONFIG_HOME when set", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-config")
		got, err := DefaultConfigDir()
		require.NoError(t, err)
		assert.Equal(t, "/tmp/xdg-config/crumbs", got)
	})

	t.Run("falls back to ~/.config when XDG unset", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "")
		home, err := os.UserHomeDir()
		require.NoError(t, err)

		got, err := DefaultConfigDir()
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(home, ".config", "crumbs"), got)
	})
}

func TestDefaultDataDir_Linux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only test")
	}

	t.Run("uses XDG_DATA_HOME when set", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "/tmp/xdg-data")
		got, err := DefaultDataDir()
		require.NoError(t, err)
		assert.Equal(t, "/tmp/xdg-data/crumbs", got)
	})

	t.Run("falls back to ~/.local/share when XDG unset", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "")
		home, err := os.UserHomeDir()
		require.NoError(t, err)

		got, err := DefaultDataDir()
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(home, ".local", "share", "crumbs"), got)
	})
}

func TestDefaultConfigDir_Darwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-only test")
	}

	got, err := DefaultConfigDir()
	require.NoError(t, err)

	home, err := os.UserHomeDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, "Library", "Application Support", "crumbs"), got)
}

func TestDefaultDataDir_Darwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-only test")
	}

	got, err := DefaultDataDir()
	require.NoError(t, err)

	home, err := os.UserHomeDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, "Library", "Application Support", "crumbs"), got)
}

func TestResolveConfigDir(t *testing.T) {
	tests := []struct {
		name    string
		flag    string
		envVal  string
		wantSub string // substring the result must contain
	}{
		{
			name:    "flag wins over env",
			flag:    "/explicit/config",
			envVal:  "/env/config",
			wantSub: "/explicit/config",
		},
		{
			name:    "env wins when flag empty",
			flag:    "",
			envVal:  "/env/config",
			wantSub: "/env/config",
		},
		{
			name:    "platform default when both empty",
			flag:    "",
			envVal:  "",
			wantSub: "crumbs",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(EnvConfigDir, tt.envVal)
			got, err := ResolveConfigDir(tt.flag)
			require.NoError(t, err)
			assert.Contains(t, got, tt.wantSub)
		})
	}
}

func TestResolveDataDir(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)
	cwdDefault := filepath.Join(cwd, DefaultDataDirName)

	tests := []struct {
		name           string
		flag           string
		configYAMLVal  string
		envVal         string
		want           string
		wantContains   string // use instead of want for partial match
	}{
		{
			name:         "flag wins over all",
			flag:         "/flag/data",
			configYAMLVal: "/config/data",
			envVal:       "/env/data",
			want:         "/flag/data",
		},
		{
			name:         "config.yaml wins over env",
			flag:         "",
			configYAMLVal: "/config/data",
			envVal:       "/env/data",
			want:         "/config/data",
		},
		{
			name:         "env wins when flag and config empty",
			flag:         "",
			configYAMLVal: "",
			envVal:       "/env/data",
			want:         "/env/data",
		},
		{
			name:         "CWD default when all empty",
			flag:         "",
			configYAMLVal: "",
			envVal:       "",
			want:         cwdDefault,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(EnvDataDir, tt.envVal)
			got, err := ResolveDataDir(tt.flag, tt.configYAMLVal)
			require.NoError(t, err)
			if tt.wantContains != "" {
				assert.Contains(t, got, tt.wantContains)
			} else {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestResolveConfigDir_AbsolutePath(t *testing.T) {
	t.Run("relative flag becomes absolute", func(t *testing.T) {
		t.Setenv(EnvConfigDir, "")
		got, err := ResolveConfigDir("relative/path")
		require.NoError(t, err)
		assert.True(t, filepath.IsAbs(got), "expected absolute path, got %s", got)
	})

	t.Run("relative env becomes absolute", func(t *testing.T) {
		t.Setenv(EnvConfigDir, "relative/env")
		got, err := ResolveConfigDir("")
		require.NoError(t, err)
		assert.True(t, filepath.IsAbs(got), "expected absolute path, got %s", got)
	})
}

func TestResolveDataDir_AbsolutePath(t *testing.T) {
	t.Run("relative flag becomes absolute", func(t *testing.T) {
		t.Setenv(EnvDataDir, "")
		got, err := ResolveDataDir("relative/path", "")
		require.NoError(t, err)
		assert.True(t, filepath.IsAbs(got), "expected absolute path, got %s", got)
	})

	t.Run("relative config value becomes absolute", func(t *testing.T) {
		t.Setenv(EnvDataDir, "")
		got, err := ResolveDataDir("", "relative/config")
		require.NoError(t, err)
		assert.True(t, filepath.IsAbs(got), "expected absolute path, got %s", got)
	})
}
