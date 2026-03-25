package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnsureNodeCfg(t *testing.T) {
	t.Run("Skips Existing File", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "node.cfg")
		require.NoError(t, os.WriteFile(path, []byte("[manager]\ntype=manager\n"), 0600))

		require.NoError(t, ensureNodeCfg(path))

		content, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, "[manager]\ntype=manager\n", string(content))
	})

	t.Run("Detects Empty File", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "node.cfg")
		require.NoError(t, os.WriteFile(path, []byte(""), 0600))

		// fails because stdin is not interactive
		require.Error(t, ensureNodeCfg(path))
	})

	t.Run("Detects Missing File", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "node.cfg")

		require.Error(t, ensureNodeCfg(path))
	})
}

func TestResolveConfig(t *testing.T) {
	t.Run("Defaults", func(t *testing.T) {
		t.Setenv("ZEEK_TOP_DIR", "")
		t.Setenv("ZEEK_RELEASE", "")
		t.Setenv("zeek_top_dir", "")
		t.Setenv("zeek_release", "")

		image, hostDir := resolveConfig()
		require.Equal(t, "/opt/zeek", hostDir)
		require.Equal(t, "activecm/zeek:latest", image)
	})

	t.Run("Uppercase Env Vars", func(t *testing.T) {
		t.Setenv("ZEEK_TOP_DIR", "/usr/local/zeek")
		t.Setenv("ZEEK_RELEASE", "6.2.1")
		t.Setenv("zeek_top_dir", "")
		t.Setenv("zeek_release", "")

		image, hostDir := resolveConfig()
		require.Equal(t, "/usr/local/zeek", hostDir)
		require.Equal(t, "activecm/zeek:6.2.1", image)
	})

	t.Run("Legacy Lowercase Fallback", func(t *testing.T) {
		t.Setenv("ZEEK_TOP_DIR", "")
		t.Setenv("ZEEK_RELEASE", "")
		t.Setenv("zeek_top_dir", "/legacy/path")
		t.Setenv("zeek_release", "5.0.0")

		image, hostDir := resolveConfig()
		require.Equal(t, "/legacy/path", hostDir)
		require.Equal(t, "activecm/zeek:5.0.0", image)
	})

	t.Run("Uppercase Takes Priority", func(t *testing.T) {
		t.Setenv("ZEEK_TOP_DIR", "/new/path")
		t.Setenv("ZEEK_RELEASE", "7.0.0")
		t.Setenv("zeek_top_dir", "/old/path")
		t.Setenv("zeek_release", "6.2.1")

		image, hostDir := resolveConfig()
		require.Equal(t, "/new/path", hostDir)
		require.Equal(t, "activecm/zeek:7.0.0", image)
	})
}
