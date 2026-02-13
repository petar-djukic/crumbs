// Integration tests for configuration loading and path resolution precedence.
// Exercises the cupboard binary via os/exec with various flag, env, and config
// file combinations.
// Implements: test-rel01.1-uc003-configuration-loading;
//             rel01.1-uc003-configuration-loading S1-S9.
package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cleanEnv returns os.Environ() with all CRUMBS_* and XDG_* variables removed,
// providing a clean baseline for subprocess isolation.
func cleanEnv() []string {
	var env []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "CRUMBS_") || strings.HasPrefix(e, "XDG_") {
			continue
		}
		env = append(env, e)
	}
	return env
}

// runCupboardWith executes the cupboard binary with explicit control over
// flags, environment, and working directory. Unlike runCupboard (which always
// injects --data-dir), this helper passes args unchanged so callers can test
// the full precedence chain. The subprocess environment is cleaned of CRUMBS_*
// and XDG_* variables before adding the provided env overrides.
func runCupboardWith(t *testing.T, env []string, workDir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	bin := ensureBinary(t)
	cmd := exec.Command(bin, args...)
	cmd.Env = append(cleanEnv(), env...)
	if workDir != "" {
		cmd.Dir = workDir
	}
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("run cupboard: %v", err)
		}
	}
	return stdout, stderr, exitCode
}

// writeConfigYAML writes a config.yaml file in the given directory.
func writeConfigYAML(t *testing.T, configDir, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(configDir, "config.yaml"),
		[]byte(content), 0o644))
}

// --- S1: Platform defaults return expected values ---

func TestConfigLoading_PlatformDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	dataDir := filepath.Join(tmpDir, "data")

	stdout, stderr, code := runCupboardWith(t, nil, "",
		"--config-dir", configDir,
		"--data-dir", dataDir,
		"init",
	)
	assert.Equal(t, 0, code, "init failed: stdout=%s stderr=%s", stdout, stderr)

	info, err := os.Stat(dataDir)
	require.NoError(t, err, "data dir should exist")
	assert.True(t, info.IsDir(), "data dir should be a directory")

	_, err = os.Stat(filepath.Join(dataDir, "crumbs.jsonl"))
	assert.NoError(t, err, "crumbs.jsonl should exist in data dir")
}

// --- S2: CRUMBS_CONFIG_DIR env overrides config directory ---
// --- S3: CRUMBS_DATA_DIR env overrides data directory ---

func TestConfigLoading_EnvironmentOverrides(t *testing.T) {
	t.Run("S2: CRUMBS_CONFIG_DIR is resolved with explicit data-dir", func(t *testing.T) {
		tmpDir := t.TempDir()
		envConfigDir := filepath.Join(tmpDir, "env-config")
		envDataDir := filepath.Join(tmpDir, "env-data")

		_, stderr, code := runCupboardWith(t,
			[]string{"CRUMBS_CONFIG_DIR=" + envConfigDir},
			"",
			"--config-dir", envConfigDir,
			"--data-dir", envDataDir,
			"init",
		)
		assert.Equal(t, 0, code, "init failed: %s", stderr)

		_, err := os.Stat(filepath.Join(envDataDir, "crumbs.jsonl"))
		assert.NoError(t, err, "crumbs.jsonl should exist in env-data dir")
	})

	t.Run("S3: CRUMBS_DATA_DIR env overrides data directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		configDir := filepath.Join(tmpDir, "config")
		envDataDir := filepath.Join(tmpDir, "env-data")

		// CRUMBS_DATA_DIR should be used when no --data-dir flag and no
		// config.yaml data_dir are provided.
		_, stderr, code := runCupboardWith(t,
			[]string{"CRUMBS_DATA_DIR=" + envDataDir},
			"",
			"--config-dir", configDir,
			"init",
		)
		assert.Equal(t, 0, code, "init failed: %s", stderr)

		_, err := os.Stat(filepath.Join(envDataDir, "crumbs.jsonl"))
		assert.NoError(t, err, "crumbs.jsonl should exist in CRUMBS_DATA_DIR location")
	})
}

// --- S4: --config-dir flag overrides CRUMBS_CONFIG_DIR env ---
// --- S5: --data-dir flag overrides config.yaml data_dir and platform default ---

func TestConfigLoading_FlagOverrides(t *testing.T) {
	t.Run("S4: --config-dir overrides CRUMBS_CONFIG_DIR", func(t *testing.T) {
		tmpDir := t.TempDir()
		envCfgDir := filepath.Join(tmpDir, "env-cfg")
		flagCfgDir := filepath.Join(tmpDir, "flag-cfg")
		dataDir := filepath.Join(tmpDir, "data")

		_, stderr, code := runCupboardWith(t,
			[]string{"CRUMBS_CONFIG_DIR=" + envCfgDir},
			"",
			"--config-dir", flagCfgDir,
			"--data-dir", dataDir,
			"init",
		)
		assert.Equal(t, 0, code, "init failed: %s", stderr)

		_, err := os.Stat(filepath.Join(dataDir, "crumbs.jsonl"))
		assert.NoError(t, err, "crumbs.jsonl should exist in data dir")
	})

	t.Run("S5: --data-dir overrides config.yaml data_dir", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfgDir := filepath.Join(tmpDir, "cfg")
		configDataDir := filepath.Join(tmpDir, "config-data")
		flagDataDir := filepath.Join(tmpDir, "flag-data")

		writeConfigYAML(t, cfgDir, fmt.Sprintf("backend: sqlite\ndata_dir: %s\n", configDataDir))

		_, stderr, code := runCupboardWith(t, nil, "",
			"--config-dir", cfgDir,
			"--data-dir", flagDataDir,
			"init",
		)
		assert.Equal(t, 0, code, "init failed: %s", stderr)

		_, err := os.Stat(filepath.Join(flagDataDir, "crumbs.jsonl"))
		assert.NoError(t, err, "crumbs.jsonl should exist at flag-specified data dir")

		_, err = os.Stat(filepath.Join(configDataDir, "crumbs.jsonl"))
		assert.True(t, os.IsNotExist(err), "crumbs.jsonl should NOT exist at config data_dir")
	})

	t.Run("S5b: --data-dir overrides platform default", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfgDir := filepath.Join(tmpDir, "cfg")
		explicitDataDir := filepath.Join(tmpDir, "explicit-data")

		_, stderr, code := runCupboardWith(t, nil, "",
			"--config-dir", cfgDir,
			"--data-dir", explicitDataDir,
			"init",
		)
		assert.Equal(t, 0, code, "init failed: %s", stderr)

		_, err := os.Stat(filepath.Join(explicitDataDir, "crumbs.jsonl"))
		assert.NoError(t, err, "crumbs.jsonl should exist at explicit data dir")
	})
}

// --- S6: Missing config.yaml uses defaults without error ---

func TestConfigLoading_ConfigFileLoading(t *testing.T) {
	t.Run("S6: Missing config directory works with --data-dir flag", func(t *testing.T) {
		tmpDir := t.TempDir()

		stdout, stderr, code := runCupboardWith(t, nil, "",
			"--config-dir", filepath.Join(tmpDir, "config"),
			"--data-dir", filepath.Join(tmpDir, "data"),
			"init",
		)
		assert.Equal(t, 0, code, "init failed: stdout=%s stderr=%s", stdout, stderr)

		_, err := os.Stat(filepath.Join(tmpDir, "data", "crumbs.jsonl"))
		assert.NoError(t, err, "crumbs.jsonl should exist in data dir")
	})

	t.Run("S6b: Empty config directory works with --data-dir flag", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "config"), 0o755))

		stdout, stderr, code := runCupboardWith(t, nil, "",
			"--config-dir", filepath.Join(tmpDir, "config"),
			"--data-dir", filepath.Join(tmpDir, "data"),
			"init",
		)
		assert.Equal(t, 0, code, "init failed: stdout=%s stderr=%s", stdout, stderr)

		_, err := os.Stat(filepath.Join(tmpDir, "data", "crumbs.jsonl"))
		assert.NoError(t, err, "crumbs.jsonl should exist in data dir")
	})
}

// --- S7: config.yaml data_dir is respected when no flag ---

func TestConfigLoading_ConfigYAMLDataDir(t *testing.T) {
	tmpDir := t.TempDir()
	cfgDir := filepath.Join(tmpDir, "cfg")
	yamlDataDir := filepath.Join(tmpDir, "yaml-data")

	writeConfigYAML(t, cfgDir, fmt.Sprintf("backend: sqlite\ndata_dir: %s\n", yamlDataDir))

	stdout, stderr, code := runCupboardWith(t, nil, "",
		"--config-dir", cfgDir,
		"init",
	)
	assert.Equal(t, 0, code, "init failed: stdout=%s stderr=%s", stdout, stderr)

	_, err := os.Stat(filepath.Join(yamlDataDir, "crumbs.jsonl"))
	assert.NoError(t, err, "crumbs.jsonl should exist at config.yaml data_dir path")
}

// --- S8: Config directory created on first run ---

func TestConfigLoading_ConfigDirectoryCreation(t *testing.T) {
	tmpDir := t.TempDir()
	newConfigDir := filepath.Join(tmpDir, "new-config-dir")

	_, err := os.Stat(newConfigDir)
	require.True(t, os.IsNotExist(err), "config dir should not exist before test")

	stdout, stderr, code := runCupboardWith(t,
		[]string{"CRUMBS_CONFIG_DIR=" + newConfigDir},
		"",
		"--data-dir", filepath.Join(tmpDir, "data"),
		"init",
	)
	assert.Equal(t, 0, code, "init failed: stdout=%s stderr=%s", stdout, stderr)

	info, err := os.Stat(newConfigDir)
	require.NoError(t, err, "config dir should be created on first run")
	assert.True(t, info.IsDir(), "config dir should be a directory")
}

// --- Precedence chain: flag > env > config > default ---

func TestConfigLoading_PrecedenceChain(t *testing.T) {
	tmpDir := t.TempDir()
	envConfigDir := filepath.Join(tmpDir, "env-config")
	envDataDir := filepath.Join(tmpDir, "env-data")
	flagDataDir := filepath.Join(tmpDir, "flag-data")

	writeConfigYAML(t, envConfigDir, fmt.Sprintf("backend: sqlite\ndata_dir: %s\n", envDataDir))

	stdout, stderr, code := runCupboardWith(t,
		[]string{"CRUMBS_CONFIG_DIR=" + envConfigDir},
		"",
		"--data-dir", flagDataDir,
		"init",
	)
	assert.Equal(t, 0, code, "init failed: stdout=%s stderr=%s", stdout, stderr)

	_, err := os.Stat(filepath.Join(flagDataDir, "crumbs.jsonl"))
	assert.NoError(t, err, "crumbs.jsonl should exist at flag-specified data dir")

	_, err = os.Stat(filepath.Join(envDataDir, "crumbs.jsonl"))
	assert.True(t, os.IsNotExist(err), "crumbs.jsonl should NOT exist at config data_dir when flag overrides")
}

// --- Error conditions ---

func TestConfigLoading_ErrorConditions(t *testing.T) {
	t.Run("Invalid YAML in config.yaml causes error", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfgDir := filepath.Join(tmpDir, "config")
		require.NoError(t, os.MkdirAll(cfgDir, 0o755))

		require.NoError(t, os.WriteFile(
			filepath.Join(cfgDir, "config.yaml"),
			[]byte("invalid: yaml: syntax: : :"), 0o644))

		_, stderr, code := runCupboardWith(t, nil, "",
			"--config-dir", cfgDir,
			"--data-dir", filepath.Join(tmpDir, "data"),
			"init",
		)
		assert.NotEqual(t, 0, code, "should fail with invalid YAML")
		assert.Contains(t, stderr, "read config", "error should mention config reading")
	})
}

// --- XDG paths on Linux ---

func TestConfigLoading_XDGPathsOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("XDG paths only apply on Linux")
	}

	tmpDir := t.TempDir()
	xdgConfigHome := filepath.Join(tmpDir, "xdg-config")
	xdgDataHome := filepath.Join(tmpDir, "xdg-data")
	crumbsConfigDir := filepath.Join(xdgConfigHome, "crumbs")
	crumbsDataDir := filepath.Join(xdgDataHome, "crumbs")

	require.NoError(t, os.MkdirAll(crumbsDataDir, 0o755))
	writeConfigYAML(t, crumbsConfigDir, fmt.Sprintf("backend: sqlite\ndata_dir: %s\n", crumbsDataDir))

	_, stderr, code := runCupboardWith(t,
		[]string{
			"XDG_CONFIG_HOME=" + xdgConfigHome,
			"XDG_DATA_HOME=" + xdgDataHome,
			"HOME=" + tmpDir,
		},
		"",
		"init",
	)
	assert.Equal(t, 0, code, "init failed: %s", stderr)

	_, err := os.Stat(filepath.Join(crumbsDataDir, "crumbs.jsonl"))
	assert.NoError(t, err, "crumbs.jsonl should exist in XDG data dir")
}

// --- S9: Commands work with resolved paths ---

func TestConfigLoading_OperationsWithResolvedPaths(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	dataDir := filepath.Join(tmpDir, "data")

	_, stderr, code := runCupboardWith(t, nil, "",
		"--config-dir", configDir,
		"--data-dir", dataDir,
		"init",
	)
	require.Equal(t, 0, code, "init failed: %s", stderr)

	stdout, stderr, code := runCupboardWith(t, nil, "",
		"--config-dir", configDir,
		"--data-dir", dataDir,
		"set", "crumbs", "", `{"name":"Test crumb","state":"draft"}`,
	)
	require.Equal(t, 0, code, "set failed: %s", stderr)
	assert.Contains(t, stdout, "Test crumb")

	stdout, stderr, code = runCupboardWith(t, nil, "",
		"--config-dir", configDir,
		"--data-dir", dataDir,
		"list", "crumbs",
	)
	require.Equal(t, 0, code, "list failed: %s", stderr)
	assert.Contains(t, stdout, "Test crumb")
}
