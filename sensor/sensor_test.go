package sensor

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenderNodeCfg(t *testing.T) {
	t.Run("Basic Config", func(t *testing.T) {
		cfg := &NodeConfig{Interface: "eth0", Workers: 4}
		result, err := RenderNodeCfg(cfg)
		require.NoError(t, err)
		require.Contains(t, result, "af_packet::eth0")
		require.Contains(t, result, "lb_procs=4")
	})

	t.Run("Contains All Sections", func(t *testing.T) {
		cfg := &NodeConfig{Interface: "eth0", Workers: 2}
		result, err := RenderNodeCfg(cfg)
		require.NoError(t, err)
		for _, section := range []string{"[manager]", "[proxy-1]", "[worker-1]"} {
			require.Contains(t, result, section)
		}
	})

	t.Run("Single Worker", func(t *testing.T) {
		cfg := &NodeConfig{Interface: "ens192", Workers: 1}
		result, err := RenderNodeCfg(cfg)
		require.NoError(t, err)
		require.Contains(t, result, "af_packet::ens192")
		require.Contains(t, result, "lb_procs=1")
	})

	t.Run("Contains AF Packet Settings", func(t *testing.T) {
		cfg := &NodeConfig{Interface: "eth0", Workers: 2}
		result, err := RenderNodeCfg(cfg)
		require.NoError(t, err)
		for _, s := range []string{
			"af_packet_fanout_id=23",
			"af_packet_fanout_mode=AF_Packet::FANOUT_HASH",
			"af_packet_buffer_size=128*1024*1024",
		} {
			require.Contains(t, result, s)
		}
	})
}

func TestGenerateNodeCfg(t *testing.T) {
	t.Run("Writes File", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "node.cfg")

		cfg := &NodeConfig{Interface: "eth0", Workers: 4}
		require.NoError(t, GenerateNodeCfg(cfg, path))

		content, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Contains(t, string(content), "af_packet::eth0")
	})

	t.Run("File Permissions", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "node.cfg")

		cfg := &NodeConfig{Interface: "eth0", Workers: 2}
		require.NoError(t, GenerateNodeCfg(cfg, path))

		info, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0640), info.Mode().Perm())
	})
}

func TestListInterfaces(t *testing.T) {
	t.Run("Excludes Loopback", func(t *testing.T) {
		ifaces, err := ListInterfaces()
		require.NoError(t, err)
		require.NotContains(t, ifaces, "lo")
	})
}

func TestPromptSelection(t *testing.T) {
	t.Run("Valid Selection", func(t *testing.T) {
		reader := bufio.NewReader(strings.NewReader("2\n"))
		result, err := promptSelection(reader, "Pick one", 3)
		require.NoError(t, err)
		require.Equal(t, 2, result)
	})

	t.Run("Rejects Zero", func(t *testing.T) {
		reader := bufio.NewReader(strings.NewReader("0\n1\n"))
		result, err := promptSelection(reader, "Pick one", 3)
		require.NoError(t, err)
		require.Equal(t, 1, result)
	})

	t.Run("Rejects Out Of Range", func(t *testing.T) {
		reader := bufio.NewReader(strings.NewReader("5\n2\n"))
		result, err := promptSelection(reader, "Pick one", 3)
		require.NoError(t, err)
		require.Equal(t, 2, result)
	})

	t.Run("Rejects Non Numeric", func(t *testing.T) {
		reader := bufio.NewReader(strings.NewReader("abc\n1\n"))
		result, err := promptSelection(reader, "Pick one", 3)
		require.NoError(t, err)
		require.Equal(t, 1, result)
	})
}
