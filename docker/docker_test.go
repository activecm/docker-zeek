package docker

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildRunArgs(t *testing.T) {
	t.Run("Contains Required Docker Flags", func(t *testing.T) {
		args := buildRunArgs("activecm/zeek:8.0.6", "/opt/zeek", "unless-stopped", nil)

		require.Subset(t, args, []string{
			"--detach", "--network", "host",
			"--cap-add", "net_raw", "net_admin",
			"--ulimit", "nofile=1048576:1048576",
			"--restart", "unless-stopped",
			"--name", ContainerName,
		})
	})

	t.Run("Image Is Last Arg", func(t *testing.T) {
		args := buildRunArgs("activecm/zeek:8.0.6", "/opt/zeek", "unless-stopped", nil)
		require.Equal(t, "activecm/zeek:8.0.6", args[len(args)-1])
	})

	t.Run("Contains Bind Mounts", func(t *testing.T) {
		args := buildRunArgs("activecm/zeek:8.0.6", "/opt/zeek", "unless-stopped", nil)

		require.Subset(t, args, []string{
			"source=/etc/localtime,destination=/etc/localtime,type=bind,readonly",
			"source=/opt/zeek/logs,destination=/usr/local/zeek/logs/,type=bind",
			"source=/opt/zeek/spool,destination=/usr/local/zeek/spool/,type=bind",
		})
	})

	t.Run("Passes Env Vars", func(t *testing.T) {
		args := buildRunArgs("activecm/zeek:8.0.6", "/opt/zeek", "unless-stopped", map[string]string{
			"ZEEK_INTERFACE": "eth0",
		})
		require.Contains(t, args, "ZEEK_INTERFACE=eth0")
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

	t.Run("Disables Notice Sendmail", func(t *testing.T) {
		require.Contains(t, buildReadPCAPCommand(), "Notice::sendmail = ")
	})

	t.Run("Filters Node Names Warning", func(t *testing.T) {
		require.Contains(t, buildReadPCAPCommand(), "Node names are not added to logs")
	})
}

func TestValidatePath(t *testing.T) {
	t.Run("Valid Path", func(t *testing.T) {
		require.NoError(t, ValidatePath("/opt/zeek"))
	})

	t.Run("Valid Path With Periods In Names", func(t *testing.T) {
		require.NoError(t, ValidatePath("/opt/zeek/conn.log"))
		require.NoError(t, ValidatePath("/opt/.config/zeek"))
	})

	t.Run("Valid Path With Doubled Slashes", func(t *testing.T) {
		require.NoError(t, ValidatePath("/opt//zeek"))
	})

	t.Run("Rejects Empty", func(t *testing.T) {
		require.ErrorIs(t, ValidatePath(""), ErrInvalidPath)
	})

	t.Run("Rejects Whitespace Only", func(t *testing.T) {
		require.ErrorIs(t, ValidatePath("   "), ErrInvalidPath)
	})

	t.Run("Rejects Relative Paths", func(t *testing.T) {
		require.ErrorIs(t, ValidatePath("relative/path"), ErrInvalidPath)
	})

	t.Run("Rejects Traversal", func(t *testing.T) {
		require.ErrorIs(t, ValidatePath("/opt/zeek/../etc"), ErrInvalidPath)
	})

	t.Run("Rejects Traversal At Start", func(t *testing.T) {
		require.ErrorIs(t, ValidatePath("/../etc"), ErrInvalidPath)
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
		require.Equal(t,
			[]string{"--mount", "source=/src,destination=/dest,type=bind,readonly"},
			bindMount("/src", "/dest", true))
	})

	t.Run("Writable", func(t *testing.T) {
		require.Equal(t,
			[]string{"--mount", "source=/src,destination=/dest,type=bind"},
			bindMount("/src", "/dest", false))
	})
}

func TestFindAndMount(t *testing.T) {
	t.Run("Mounts All Files", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "networks.cfg"), []byte("test"), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "zeekctl.cfg"), []byte("test"), 0600))

		require.Equal(t, 2, countMounts(findAndMount(dir, dir, false)))
	})

	t.Run("Skips Directories", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "subdir"), 0750))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "file.cfg"), []byte("test"), 0600))

		require.Equal(t, 1, countMounts(findAndMount(dir, dir, false)))
	})

	t.Run("Skips Symlinks", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "file.cfg"), []byte("test"), 0600))
		require.NoError(t, os.Symlink(
			filepath.Join(dir, "file.cfg"),
			filepath.Join(dir, "symlink.cfg")))

		args := findAndMount(dir, dir, false)
		require.Equal(t, 1, countMounts(args))
		require.NotContains(t, strings.Join(args, " "), "symlink.cfg")
	})
}

func TestFindAndMountZeekScripts(t *testing.T) {
	t.Run("Mounts Zeek Files", func(t *testing.T) {
		dir := t.TempDir()
		autoload := filepath.Join(dir, "share/zeek/site/autoload")
		require.NoError(t, os.MkdirAll(autoload, 0750))
		require.NoError(t, os.WriteFile(filepath.Join(autoload, "100-default.zeek"), []byte("test"), 0600))

		args := findAndMountZeekScripts(dir)
		require.Equal(t, 1, countMounts(args))
		require.Contains(t, strings.Join(args, " "), "100-default.zeek")
	})

	t.Run("Skips Local Zeek", func(t *testing.T) {
		dir := t.TempDir()
		autoload := filepath.Join(dir, "share/zeek/site/autoload")
		require.NoError(t, os.MkdirAll(autoload, 0750))
		require.NoError(t, os.WriteFile(filepath.Join(autoload, "local.zeek"), []byte("test"), 0600))

		require.Empty(t, findAndMountZeekScripts(dir))
	})

	t.Run("Skips Non Zeek Files", func(t *testing.T) {
		dir := t.TempDir()
		autoload := filepath.Join(dir, "share/zeek/site/autoload")
		require.NoError(t, os.MkdirAll(autoload, 0750))
		require.NoError(t, os.WriteFile(filepath.Join(autoload, "readme.txt"), []byte("test"), 0600))

		require.Empty(t, findAndMountZeekScripts(dir))
	})

	t.Run("Missing Autoload Directory", func(t *testing.T) {
		require.Empty(t, findAndMountZeekScripts(t.TempDir()))
	})

	t.Run("Skips Symlinks", func(t *testing.T) {
		dir := t.TempDir()
		autoload := filepath.Join(dir, "share/zeek/site/autoload")
		require.NoError(t, os.MkdirAll(autoload, 0750))
		require.NoError(t, os.WriteFile(filepath.Join(autoload, "real.zeek"), []byte("test"), 0600))
		require.NoError(t, os.Symlink(
			filepath.Join(autoload, "real.zeek"),
			filepath.Join(autoload, "evil.zeek")))

		args := findAndMountZeekScripts(dir)
		require.Equal(t, 1, countMounts(args))
		require.NotContains(t, strings.Join(args, " "), "evil.zeek")
	})
}

func TestReadHeaderVersion(t *testing.T) {
	setup := func(t *testing.T, content string) string {
		t.Helper()
		dir := t.TempDir()
		path := filepath.Join(dir, "test.cfg")
		require.NoError(t, os.WriteFile(path, []byte(content), 0600))
		return path
	}

	t.Run("Header With Version", func(t *testing.T) {
		path := setup(t, "# Generated by docker-zeek v8.0.6\n# Customize as needed.\n")
		v, err := readHeaderVersion(path)
		require.NoError(t, err)
		require.Equal(t, "v8.0.6", v)
	})

	t.Run("No Header", func(t *testing.T) {
		path := setup(t, "MailTo = root@localhost\n")
		v, err := readHeaderVersion(path)
		require.NoError(t, err)
		require.Equal(t, "", v)
	})

	t.Run("Empty File", func(t *testing.T) {
		path := setup(t, "")
		v, err := readHeaderVersion(path)
		require.NoError(t, err)
		require.Equal(t, "", v)
	})

	t.Run("Blank Lines Before Header", func(t *testing.T) {
		path := setup(t, "\n\n# Generated by docker-zeek v8.0.6\n")
		v, err := readHeaderVersion(path)
		require.NoError(t, err)
		require.Equal(t, "v8.0.6", v)
	})

	t.Run("CRLF Line Endings", func(t *testing.T) {
		path := setup(t, "# Generated by docker-zeek v8.0.6\r\n# Customize as needed.\r\n")
		v, err := readHeaderVersion(path)
		require.NoError(t, err)
		require.Equal(t, "v8.0.6", v)
	})

	t.Run("Whitespace Around Header", func(t *testing.T) {
		path := setup(t, "  # Generated by docker-zeek v8.0.6  \n")
		v, err := readHeaderVersion(path)
		require.NoError(t, err)
		require.Equal(t, "v8.0.6", v)
	})

	t.Run("Special Chars In Version", func(t *testing.T) {
		path := setup(t, "# Generated by docker-zeek v8.0.6-3-gabc1234\n")
		v, err := readHeaderVersion(path)
		require.NoError(t, err)
		require.Equal(t, "v8.0.6-3-gabc1234", v)
	})

	t.Run("Dev Version", func(t *testing.T) {
		path := setup(t, "# Generated by docker-zeek dev\n")
		v, err := readHeaderVersion(path)
		require.NoError(t, err)
		require.Equal(t, "dev", v)
	})

	t.Run("Missing File", func(t *testing.T) {
		_, err := readHeaderVersion("/nonexistent/path/file")
		require.ErrorIs(t, err, fs.ErrNotExist)
	})
}

func TestHandleUserEditable(t *testing.T) {
	setup := func(t *testing.T) (string, string) {
		t.Helper()
		dir := t.TempDir()
		etcDir := filepath.Join(dir, "etc")
		require.NoError(t, os.MkdirAll(etcDir, 0750))
		return dir, filepath.Join(etcDir, "zeekctl.cfg")
	}

	t.Run("Current Version", func(t *testing.T) {
		dir, path := setup(t)
		original := []byte("# Generated by docker-zeek v8.0.6\n# Customize as needed.\n[manager]\n")
		require.NoError(t, os.WriteFile(path, original, 0600))

		err := handleUserEditable("nonexistent-container", dir, "v8.0.6",
			"etc/zeekctl.cfg", "/usr/local/zeek/etc/zeekctl.cfg")
		require.NoError(t, err)

		content, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, original, content)
		require.NoFileExists(t, path+".bak")
		require.NoFileExists(t, path+".new")
	})

	t.Run("No Header", func(t *testing.T) {
		dir, path := setup(t)
		legacy := []byte("MailTo = root@localhost\ninterfacesetup.enabled=1\n")
		require.NoError(t, os.WriteFile(path, legacy, 0600))

		// rename happens first, then cp fails because container is fake
		_ = handleUserEditable("nonexistent-container", dir, "v8.0.6",
			"etc/zeekctl.cfg", "/usr/local/zeek/etc/zeekctl.cfg")

		require.NoFileExists(t, path)
		require.FileExists(t, path+".bak")
		archived, err := os.ReadFile(path + ".bak")
		require.NoError(t, err)
		require.Equal(t, legacy, archived)
	})

	t.Run("Version Mismatch", func(t *testing.T) {
		dir, path := setup(t)
		oldContent := []byte("# Generated by docker-zeek v8.0.5\n# Customize as needed.\n")
		require.NoError(t, os.WriteFile(path, oldContent, 0600))

		// goes straight to cp .new, which fails. Live file untouched.
		_ = handleUserEditable("nonexistent-container", dir, "v8.0.6",
			"etc/zeekctl.cfg", "/usr/local/zeek/etc/zeekctl.cfg")

		require.FileExists(t, path)
		content, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, oldContent, content)
		require.NoFileExists(t, path+".bak")
	})
}

func TestHandleSystemManaged(t *testing.T) {
	setup := func(t *testing.T) (string, string) {
		t.Helper()
		dir := t.TempDir()
		autoload := filepath.Join(dir, "share", "zeek", "site", "autoload")
		require.NoError(t, os.MkdirAll(autoload, 0750))
		return dir, filepath.Join(autoload, "200-inactivity_timeout.zeek")
	}

	t.Run("Current Version", func(t *testing.T) {
		dir, path := setup(t)
		original := []byte("# Generated by docker-zeek v8.0.6\n# Do not edit directly.\nredef tcp_inactivity_timeout = 60 min;\n")
		require.NoError(t, os.WriteFile(path, original, 0600))

		err := handleSystemManaged("nonexistent-container", dir, "v8.0.6",
			"share/zeek/site/autoload/200-inactivity_timeout.zeek",
			"/usr/local/zeek/share/zeek/site/autoload/200-inactivity_timeout.zeek")
		require.NoError(t, err)

		content, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, original, content)
	})

	t.Run("Version Mismatch", func(t *testing.T) {
		dir, path := setup(t)
		require.NoError(t, os.WriteFile(path, []byte("# Generated by docker-zeek v8.0.5\n# Do not edit directly.\n"), 0600))

		_ = handleSystemManaged("nonexistent-container", dir, "v8.0.6",
			"share/zeek/site/autoload/200-inactivity_timeout.zeek",
			"/usr/local/zeek/share/zeek/site/autoload/200-inactivity_timeout.zeek")

		require.FileExists(t, path)
		require.NoFileExists(t, path+".bak")
		require.NoFileExists(t, path+".new")
	})

	t.Run("No Header", func(t *testing.T) {
		dir, path := setup(t)
		require.NoError(t, os.WriteFile(path, []byte("redef tcp_inactivity_timeout = 30 min;\n"), 0600))

		_ = handleSystemManaged("nonexistent-container", dir, "v8.0.6",
			"share/zeek/site/autoload/200-inactivity_timeout.zeek",
			"/usr/local/zeek/share/zeek/site/autoload/200-inactivity_timeout.zeek")

		require.FileExists(t, path)
		require.NoFileExists(t, path+".bak")
		require.NoFileExists(t, path+".new")
	})
}

func TestArchiveLocalZeek(t *testing.T) {
	setup := func(t *testing.T) (string, string) {
		t.Helper()
		dir := t.TempDir()
		siteDir := filepath.Join(dir, "share", "zeek", "site")
		require.NoError(t, os.MkdirAll(siteDir, 0755))
		return dir, filepath.Join(siteDir, "local.zeek")
	}

	t.Run("Skips When Missing", func(t *testing.T) {
		dir, path := setup(t)
		require.NoError(t, archiveLocalZeek(dir))
		require.NoFileExists(t, path)
		require.NoFileExists(t, path+".bak")
	})

	t.Run("Skips When Empty", func(t *testing.T) {
		dir, path := setup(t)
		require.NoError(t, os.WriteFile(path, []byte(""), 0600))
		require.NoError(t, archiveLocalZeek(dir))
		require.FileExists(t, path)
		require.NoFileExists(t, path+".bak")
	})

	t.Run("Archives When Present", func(t *testing.T) {
		dir, path := setup(t)
		original := "@load my-script\n"
		require.NoError(t, os.WriteFile(path, []byte(original), 0600))

		require.NoError(t, archiveLocalZeek(dir))

		require.NoFileExists(t, path)
		archived, err := os.ReadFile(path + ".bak")
		require.NoError(t, err)
		require.Equal(t, original, string(archived))
	})
}

func countMounts(args []string) int {
	n := 0
	for _, a := range args {
		if a == "--mount" {
			n++
		}
	}
	return n
}
