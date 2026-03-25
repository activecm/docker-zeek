package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildRunArgs(t *testing.T) {
	t.Run("Contains Required Docker Flags", func(t *testing.T) {
		args := buildRunArgs("activecm/zeek:8.0.6", "/opt/zeek", "always")

		require.Subset(t, args, []string{
			"--detach", "--network", "host",
			"--cap-add", "net_raw", "net_admin",
			"--ulimit", "nofile=1048576:1048576",
			"--restart", "always",
			"--name", ContainerName,
		})
	})

	t.Run("Image Is Last Arg", func(t *testing.T) {
		args := buildRunArgs("activecm/zeek:8.0.6", "/opt/zeek", "always")
		require.Equal(t, "activecm/zeek:8.0.6", args[len(args)-1])
	})

	t.Run("Contains Volume Mounts", func(t *testing.T) {
		args := buildRunArgs("activecm/zeek:8.0.6", "/opt/zeek", "always")

		require.Subset(t, args, []string{
			"source=zeek-zkg-script,destination=/usr/local/zeek/share/zeek/site/packages/,type=volume",
			"source=zeek-zkg-plugin,destination=/usr/local/zeek/lib/zeek/plugins/packages/,type=volume",
			"source=zeek-zkg-state,destination=/root/.zkg,type=volume",
		})
	})

	t.Run("Contains Bind Mounts", func(t *testing.T) {
		args := buildRunArgs("activecm/zeek:8.0.6", "/opt/zeek", "always")

		require.Subset(t, args, []string{
			"source=/etc/localtime,destination=/etc/localtime,type=bind,readonly",
			"source=/opt/zeek/logs,destination=/usr/local/zeek/logs/,type=bind",
			"source=/opt/zeek/spool,destination=/usr/local/zeek/spool/,type=bind",
		})
	})
}

func TestBuildReadPCAPArgs(t *testing.T) {
	t.Run("Contains Required Docker Flags", func(t *testing.T) {
		args := buildReadPCAPArgs("activecm/zeek:8.0.6", "/opt/zeek", "/tmp/test.pcap", "/tmp/logs")
		require.Subset(t, args, []string{"--rm", "--workdir", "--entrypoint", "/bin/bash"})
	})

	t.Run("Mounts Pcap File Readonly", func(t *testing.T) {
		args := buildReadPCAPArgs("activecm/zeek:8.0.6", "/opt/zeek", "/tmp/test.pcap", "/tmp/logs")
		require.Contains(t, args, "source=/tmp/test.pcap,destination=/incoming.pcap,type=bind,readonly")
	})

	t.Run("Mounts Log Output Directory", func(t *testing.T) {
		args := buildReadPCAPArgs("activecm/zeek:8.0.6", "/opt/zeek", "/tmp/test.pcap", "/tmp/logs")
		require.Contains(t, args, "source=/tmp/logs,destination=/usr/local/zeek/logs/,type=bind")
	})
}

func TestBuildReadPCAPCommand(t *testing.T) {
	t.Run("Generates Local Zeek Config", func(t *testing.T) {
		require.Contains(t, buildReadPCAPCommand(), "local.zeek")
	})

	t.Run("References Pcap File", func(t *testing.T) {
		require.Contains(t, buildReadPCAPCommand(), "-r /incoming.pcap")
	})

	t.Run("Sets Local Nets", func(t *testing.T) {
		require.Contains(t, buildReadPCAPCommand(), "Site::local_nets")
	})
}

func TestVolumeMount(t *testing.T) {
	t.Run("Returns Mount Flag And Value", func(t *testing.T) {
		result := volumeMount("myvolume", "/data")
		require.Len(t, result, 2)
		require.Equal(t, "--mount", result[0])
		require.Equal(t, "source=myvolume,destination=/data,type=volume", result[1])
	})
}

func TestValidatePath(t *testing.T) {
	t.Run("Valid Path", func(t *testing.T) {
		require.NoError(t, ValidatePath("/opt/zeek"))
	})

	t.Run("Rejects Commas", func(t *testing.T) {
		require.ErrorIs(t, ValidatePath("/path,with,commas"), ErrInvalidPath)
	})

	t.Run("Rejects Equals", func(t *testing.T) {
		require.ErrorIs(t, ValidatePath("/path=with=equals"), ErrInvalidPath)
	})
}

func TestBindMount(t *testing.T) {
	t.Run("Readonly", func(t *testing.T) {
		result := bindMount("/src", "/dest", true)
		require.Equal(t, "source=/src,destination=/dest,type=bind,readonly", result[1])
	})

	t.Run("Writable", func(t *testing.T) {
		result := bindMount("/src", "/dest", false)
		require.Equal(t, "source=/src,destination=/dest,type=bind", result[1])
	})
}

func TestFindAndMount(t *testing.T) {
	t.Run("Mounts All Files", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, filepath.Join(dir, "networks.cfg"), []byte("test"))
		writeTestFile(t, filepath.Join(dir, "zeekctl.cfg"), []byte("test"))

		args := findAndMount(dir, dir, false)
		require.Len(t, args, 4)
	})

	t.Run("Skips Directories", func(t *testing.T) {
		dir := t.TempDir()
		mkdirTest(t, filepath.Join(dir, "subdir"))
		writeTestFile(t, filepath.Join(dir, "file.cfg"), []byte("test"))

		args := findAndMount(dir, dir, false)
		require.Len(t, args, 2)
	})
}

func TestFindAndMountZeekScripts(t *testing.T) {
	t.Run("Mounts Zeek Files", func(t *testing.T) {
		dir := t.TempDir()
		autoload := filepath.Join(dir, "zeek", "site", "autoload")
		mkdirTest(t, autoload)
		writeTestFile(t, filepath.Join(autoload, "100-default.zeek"), []byte("test"))

		args := findAndMountZeekScripts(dir, dir)
		require.Len(t, args, 2)
		require.Contains(t, strings.Join(args, " "), "100-default.zeek")
	})

	t.Run("Skips Local Zeek", func(t *testing.T) {
		dir := t.TempDir()
		autoload := filepath.Join(dir, "zeek", "site", "autoload")
		mkdirTest(t, autoload)
		writeTestFile(t, filepath.Join(autoload, "local.zeek"), []byte("test"))

		require.Empty(t, findAndMountZeekScripts(dir, dir))
	})

	t.Run("Skips Non Zeek Files", func(t *testing.T) {
		dir := t.TempDir()
		autoload := filepath.Join(dir, "zeek", "site", "autoload")
		mkdirTest(t, autoload)
		writeTestFile(t, filepath.Join(autoload, "readme.txt"), []byte("test"))

		require.Empty(t, findAndMountZeekScripts(dir, dir))
	})

	t.Run("Empty Directory", func(t *testing.T) {
		require.Empty(t, findAndMountZeekScripts(t.TempDir(), t.TempDir()))
	})
}

func TestCopyIfMissing(t *testing.T) {
	t.Run("Skips Existing File", func(t *testing.T) {
		dir := t.TempDir()
		existing := filepath.Join(dir, "etc", "networks.cfg")
		mkdirTest(t, filepath.Join(dir, "etc"))
		writeTestFile(t, existing, []byte("existing"))

		_ = copyIfMissing("nonexistent-container", dir, "etc/networks.cfg", "/usr/local/zeek/etc/networks.cfg")

		content, err := os.ReadFile(existing)
		require.NoError(t, err)
		require.Equal(t, "existing", string(content))
	})
}

func writeTestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, content, 0600))
}

func mkdirTest(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(path, 0750))
}
