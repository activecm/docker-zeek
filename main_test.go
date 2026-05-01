package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadZeekEnv(t *testing.T) {
	t.Run("Missing File Returns Empty", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "zeek.env")
		vals, err := readZeekEnv(path)
		require.NoError(t, err)
		require.Empty(t, vals)
	})

	t.Run("Skips Comments And Blanks", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "zeek.env")
		content := "# a comment\n\nZEEK_INTERFACE=eth0\n\n# another\nZEEK_WORKERS=8\n"
		require.NoError(t, os.WriteFile(path, []byte(content), 0644))

		vals, err := readZeekEnv(path)
		require.NoError(t, err)
		require.Equal(t, map[string]string{
			"ZEEK_INTERFACE": "eth0",
			"ZEEK_WORKERS":   "8",
		}, vals)
	})

	t.Run("Trims Whitespace", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "zeek.env")
		require.NoError(t, os.WriteFile(path, []byte("  ZEEK_INTERFACE = eth0  \n"), 0644))

		vals, err := readZeekEnv(path)
		require.NoError(t, err)
		require.Equal(t, "eth0", vals["ZEEK_INTERFACE"])
	})

	t.Run("Skips Lines Without Equals", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "zeek.env")
		content := "ZEEK_INTERFACE=eth0\nbroken line\nZEEK_WORKERS=8\n"
		require.NoError(t, os.WriteFile(path, []byte(content), 0644))

		vals, err := readZeekEnv(path)
		require.NoError(t, err)
		require.Equal(t, map[string]string{
			"ZEEK_INTERFACE": "eth0",
			"ZEEK_WORKERS":   "8",
		}, vals)
	})
}

func TestWriteZeekEnv(t *testing.T) {
	t.Run("Writes All Keys", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "zeek.env")
		expected := map[string]string{
			"ZEEK_WORKERS":   "8",
			"ZEEK_INTERFACE": "eth0",
		}
		require.NoError(t, writeZeekEnv(path, expected))

		got, err := readZeekEnv(path)
		require.NoError(t, err)
		require.Equal(t, expected, got)
	})

	t.Run("Overwrites Existing File", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "zeek.env")
		require.NoError(t, os.WriteFile(path, []byte("# old comment\nZEEK_INTERFACE=old\n"), 0644))

		require.NoError(t, writeZeekEnv(path, map[string]string{"ZEEK_INTERFACE": "new"}))

		got, err := readZeekEnv(path)
		require.NoError(t, err)
		require.Equal(t, map[string]string{"ZEEK_INTERFACE": "new"}, got)
	})
}

func TestResolveConfig(t *testing.T) {
	setEnv := func(t *testing.T, vals map[string]string) {
		t.Helper()
		for _, k := range []string{"ZEEK_TOP_DIR", "zeek_top_dir"} {
			t.Setenv(k, vals[k])
		}
	}

	withBuildVars := func(t *testing.T, version, imageTag string) {
		t.Helper()
		origVersion, origTag := Version, ImageTag
		Version, ImageTag = version, imageTag
		t.Cleanup(func() { Version, ImageTag = origVersion, origTag })
	}

	t.Run("Errors When Version Empty", func(t *testing.T) {
		setEnv(t, nil)
		withBuildVars(t, "", "8.0.6")
		_, _, err := resolveConfig()
		require.ErrorIs(t, err, ErrNotBuilt)
	})

	t.Run("Errors When ImageTag Empty", func(t *testing.T) {
		setEnv(t, nil)
		withBuildVars(t, "v8.0.6", "")
		_, _, err := resolveConfig()
		require.ErrorIs(t, err, ErrNotBuilt)
	})

	t.Run("Uses ImageTag For Image", func(t *testing.T) {
		setEnv(t, nil)
		withBuildVars(t, "v8.0.6", "8.0.6")
		image, hostDir, err := resolveConfig()
		require.NoError(t, err)
		require.Equal(t, "/opt/zeek", hostDir)
		require.Equal(t, "activecm/zeek:8.0.6", image)
	})

	t.Run("ZEEK_TOP_DIR works", func(t *testing.T) {
		setEnv(t, map[string]string{
			"ZEEK_TOP_DIR": "/new/path",
			"zeek_top_dir": "/old/path",
		})
		withBuildVars(t, "v8.0.6", "8.0.6")
		_, hostDir, err := resolveConfig()
		require.NoError(t, err)
		require.Equal(t, "/new/path", hostDir)
	})

	t.Run("Legacy zeek_top_dir Works", func(t *testing.T) {
		setEnv(t, map[string]string{"zeek_top_dir": "/legacy/path"})
		withBuildVars(t, "v8.0.6", "8.0.6")
		_, hostDir, err := resolveConfig()
		require.NoError(t, err)
		require.Equal(t, "/legacy/path", hostDir)
	})
}
