package docker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

const (
	ContainerName = "zeek"
	DefaultImage  = "activecm/zeek"
)

var ErrInvalidPath = errors.New("invalid characters in path")

// ContainerState holds the running state of the zeek container.
type ContainerState struct {
	Running bool
}

// Inspect returns the state of the zeek container
func Inspect() (*ContainerState, error) {
	out, err := sudoDockerQuiet("inspect", "--format", `{"running":{{.State.Running}}}`, ContainerName)
	if err != nil {
		// docker inspect exits 1 when the container doesn't exist.
		// this checks if the output mentions "No such object" to distinguish from real errors
		if strings.Contains(out, "No such object") || strings.Contains(out, "No such container") {
			return nil, nil
		}
		return nil, fmt.Errorf("inspecting container: %w", err)
	}

	var state ContainerState
	if err := json.Unmarshal([]byte(out), &state); err != nil {
		return nil, fmt.Errorf("parsing container state: %w", err)
	}
	return &state, nil
}

// Pull pulls the given image
func Pull(image string) error {
	_, err := sudoDocker("pull", image)
	return err
}

// Stop stops and removes the zeek container
func Stop() error {
	state, err := Inspect()
	if err != nil {
		return err
	}
	if state != nil && state.Running {
		fmt.Fprintln(os.Stderr, "Stopping the Zeek docker container")
		if _, err := sudoDocker("stop", "-t", "70", ContainerName); err != nil {
			return err
		}
	} else {
		fmt.Fprintln(os.Stderr, "Zeek is already stopped.")
	}
	// remove the container regardless, ignore errors
	_, _ = sudoDocker("rm", "--force", ContainerName)
	return nil
}

// Start starts the zeek container with the given image and host directory
func Start(image, hostDir string) error {
	state, err := Inspect()
	if err != nil {
		return err
	}
	if state != nil && state.Running {
		fmt.Fprintln(os.Stderr, "Zeek is already running.")
		return nil
	}

	if err := createVolumes(); err != nil {
		return err
	}

	args := buildRunArgs(image, hostDir, "unless-stopped")
	fmt.Fprintln(os.Stderr, "Starting the Zeek docker container")
	_, err = sudoDocker(args...)
	return err
}

// ReadPCAP runs zeek against a pcap file and writes logs to the given output directory
func ReadPCAP(image, hostDir, pcapPath, logDir string) error {
	absPcap, err := filepath.Abs(pcapPath)
	if err != nil {
		return fmt.Errorf("resolving pcap path: %w", err)
	}

	absLogDir, err := filepath.Abs(logDir)
	if err != nil {
		return fmt.Errorf("resolving log directory: %w", err)
	}

	if err := createVolumes(); err != nil {
		return err
	}

	args := buildReadPCAPArgs(image, hostDir, absPcap, absLogDir)
	fmt.Fprintln(os.Stderr, "Starting the Zeek docker container")
	fmt.Fprintf(os.Stderr, "Zeek logs will be saved to %s\n", absLogDir)
	_, err = sudoDocker(args...)
	return err
}

// Status prints the status of the zeek container and zeekctl processes
func Status() error {
	fmt.Fprintln(os.Stderr, "Zeek docker container status")
	state, err := Inspect()
	if err != nil {
		return err
	}
	if state == nil || !state.Running {
		fmt.Fprintln(os.Stderr, "Zeek is not running.")
		return nil
	}
	out, err := sudoDocker("ps", "--filter", "name=^zeek$")
	if err == nil && out != "" {
		fmt.Println(out)
	}
	fmt.Fprintln(os.Stderr, "Zeek processes status")
	out, err = sudoDocker("exec", ContainerName, "zeekctl", "status")
	if err == nil && out != "" {
		fmt.Println(out)
	}
	return err
}

// InitHostDir creates the required directories and copies default config files from the container to the host directory
func InitHostDir(image, hostDir string) error {
	container := fmt.Sprintf("zeek-init-%d", os.Getpid())

	// start a temporary container
	// override the entrypoint since the entrypoint expects node.cfg which doesn't exist yet
	_, err := sudoDocker(
		"run", "--detach",
		"--ulimit", "nofile=1048576:1048576",
		"--name", container,
		"-v", hostDir+":/zeek",
		"--network", "none",
		"--entrypoint", "sh",
		image,
		"-c", "while sleep 1; do :; done",
	)
	if err != nil {
		return fmt.Errorf("starting init container: %w", err)
	}

	// ensure cleanup
	defer func() { _, _ = sudoDocker("rm", "--force", container) }()

	// create directories
	dirs := []string{
		"/zeek/manual-logs",
		"/zeek/logs",
		"/zeek/spool",
		"/zeek/etc",
		"/zeek/share/zeek/site/autoload",
	}
	mkdirArgs := append([]string{"exec", container, "mkdir", "-p"}, dirs...)
	if _, err = sudoDocker(mkdirArgs...); err != nil {
		return fmt.Errorf("creating host directories: %w", err)
	}

	// set permissions on log directories
	chmodArgs := []string{"exec", container, "chmod", "-f", "0755",
		"/zeek/manual-logs", "/zeek/logs", "/zeek/spool"}
	if _, err = sudoDocker(chmodArgs...); err != nil {
		return fmt.Errorf("setting directory permissions: %w", err)
	}

	// copy default config files if they don't exist on the host
	if err = copyIfMissing(container, hostDir, "etc/networks.cfg", "/usr/local/zeek/etc/networks.cfg"); err != nil {
		return err
	}
	if err = copyIfMissing(container, hostDir, "etc/zeekctl.cfg", "/usr/local/zeek/etc/zeekctl.cfg"); err != nil {
		return err
	}
	if err = copyIfMissing(container, hostDir, "share/zeek/site/autoload/100-default.zeek", "/usr/local/zeek/share/zeek/site/autoload/100-default.zeek"); err != nil {
		return err
	}

	// copy non-customizable autoload scripts (overwrites existing ones)
	if _, err = sudoDocker("exec", container, "bash", "-c",
		`find /usr/local/zeek/share/zeek/site/autoload/ -type f -iname '*.zeek' ! -name '100-default.zeek' -exec cp -f "{}" /zeek/share/zeek/site/autoload/ \;`); err != nil {
		return fmt.Errorf("copying autoload scripts: %w", err)
	}

	return nil
}

func copyIfMissing(container, hostDir, relPath, containerPath string) error {
	hostPath := filepath.Join(hostDir, relPath)
	if _, err := os.Stat(hostPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("checking %s: %w", relPath, err)
	}
	if _, err := sudoDocker("exec", container, "cp", "-f", containerPath, "/zeek/"+relPath); err != nil {
		return fmt.Errorf("copying %s from container: %w", relPath, err)
	}
	return nil
}

func createVolumes() error {
	volumes := []string{"zeek-zkg-script", "zeek-zkg-plugin", "zeek-zkg-state"}
	for _, v := range volumes {
		if _, err := sudoDocker("volume", "create", v); err != nil {
			return fmt.Errorf("creating volume %s: %w", v, err)
		}
	}
	return nil
}

func buildRunArgs(image, hostDir, restart string) []string {
	args := []string{
		"run", "--detach",
		"--ulimit", "nofile=1048576:1048576",
		"--name", ContainerName,
		"--restart", restart,
		"--cap-add", "net_raw",
		"--cap-add", "net_admin",
		"--network", "host",
	}

	// zkg package persistence
	args = append(args, volumeMount("zeek-zkg-script", "/usr/local/zeek/share/zeek/site/packages/")...)
	args = append(args, volumeMount("zeek-zkg-plugin", "/usr/local/zeek/lib/zeek/plugins/packages/")...)
	args = append(args, volumeMount("zeek-zkg-state", "/root/.zkg")...)

	// timezone
	args = append(args, bindMount("/etc/localtime", "/etc/localtime", true)...)

	// logs and spool
	args = append(args, bindMount(filepath.Join(hostDir, "logs"), "/usr/local/zeek/logs/", false)...)
	args = append(args, bindMount(filepath.Join(hostDir, "spool"), "/usr/local/zeek/spool/", false)...)

	// mount config files from host
	args = append(args, findAndMount(filepath.Join(hostDir, "etc"), hostDir, false)...)

	// mount zeek scripts (except local.zeek)
	args = append(args, findAndMountZeekScripts(filepath.Join(hostDir, "share"), hostDir)...)

	args = append(args, image)
	return args
}

func buildReadPCAPArgs(image, hostDir, pcapPath, logDir string) []string {
	args := []string{
		"run", "--rm",
		"--ulimit", "nofile=1048576:1048576",
		"--workdir", "/usr/local/zeek/logs/",
	}

	// zkg package persistence
	args = append(args, volumeMount("zeek-zkg-script", "/usr/local/zeek/share/zeek/site/packages/")...)
	args = append(args, volumeMount("zeek-zkg-plugin", "/usr/local/zeek/lib/zeek/plugins/packages/")...)
	args = append(args, volumeMount("zeek-zkg-state", "/root/.zkg")...)

	// timezone
	args = append(args, bindMount("/etc/localtime", "/etc/localtime", true)...)

	// output logs
	args = append(args, bindMount(logDir, "/usr/local/zeek/logs/", false)...)

	// pcap file
	args = append(args, bindMount(pcapPath, "/incoming.pcap", true)...)

	// mount config files from host
	args = append(args, findAndMount(filepath.Join(hostDir, "etc"), hostDir, false)...)

	// mount zeek scripts (except local.zeek)
	args = append(args, findAndMountZeekScripts(filepath.Join(hostDir, "share"), hostDir)...)

	// run zeek directly against the pcap
	args = append(args, "--entrypoint", "/bin/bash", image, "-c", buildReadPCAPCommand())

	return args
}

func buildReadPCAPCommand() string {
	parts := []string{
		// generate local.zeek from autoload partials
		`grep -hv '^#' /usr/local/zeek/share/zeek/site/autoload/*.zeek > /usr/local/zeek/share/zeek/site/local.zeek`,
		// disable af_packet plugin for pcap reading. subshell prevents || true from masking earlier failures.
		`(mv -f /usr/local/zeek/share/zeek/builtin-plugins/Zeek_AF_Packet/{__load__.zeek,init.zeek} /usr/local/zeek/share/zeek/builtin-plugins/ 2>/dev/null || true)`,
		// run zeek against the pcap. Notice::sendmail is set to empty to disable email alerts.
		`/usr/local/zeek/bin/zeek -C -r /incoming.pcap local 'Site::local_nets += { 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16 }' 'Notice::sendmail = '`,
	}
	return strings.Join(parts, " && ")
}

// ValidatePath checks that a path is safe to use in docker --mount flags
func ValidatePath(path string) error {
	if strings.ContainsAny(path, ",=") {
		return fmt.Errorf("%w: %q", ErrInvalidPath, path)
	}
	return nil
}

// volumeMount returns the docker run arguments to mount a named volume into the container
func volumeMount(name, dest string) []string {
	return []string{"--mount", fmt.Sprintf("source=%s,destination=%s,type=volume", name, dest)}
}

// bindMount returns the docker run arguments to bind mount a file or directory into the container
func bindMount(src, dest string, readonly bool) []string {
	mount := fmt.Sprintf("source=%s,destination=%s,type=bind", src, dest)
	if readonly {
		mount += ",readonly"
	}
	return []string{"--mount", mount}
}

// findAndMount mounts all files in a directory into the container
func findAndMount(dir, hostDir string, readonly bool) []string {
	var args []string
	entries, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		return args
	}
	for _, entry := range entries {
		info, err := os.Stat(entry)
		if err != nil || info.IsDir() {
			continue
		}
		rel, _ := filepath.Rel(hostDir, entry)
		dest := "/usr/local/zeek/" + rel
		args = append(args, bindMount(entry, dest, readonly)...)
	}
	return args
}

// findAndMountZeekScripts mounts .zeek files (except local.zeek) into the container.
func findAndMountZeekScripts(dir, hostDir string) []string {
	var args []string
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil //nolint:nilerr // skip files we can't access
		}
		lower := strings.ToLower(d.Name())
		if !strings.HasSuffix(lower, ".zeek") || d.Name() == "local.zeek" {
			return nil
		}
		rel, _ := filepath.Rel(hostDir, path)
		dest := "/usr/local/zeek/" + rel
		args = append(args, bindMount(path, dest, false)...)
		return nil
	})
	return args
}

// useSudo checks if the current user can write to the docker socket
var useSudo = sync.OnceValue(func() bool {
	f, err := os.OpenFile("/var/run/docker.sock", os.O_WRONLY, 0)
	if err != nil {
		return true
	}
	_ = f.Close()
	return false
})

// sudoDockerQuiet runs a docker command without printing stderr to the terminal
func sudoDockerQuiet(args ...string) (string, error) {
	var cmd *exec.Cmd
	if useSudo() {
		cmd = exec.Command("sudo", append([]string{"docker"}, args...)...)
	} else {
		cmd = exec.Command("docker", args...)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return strings.TrimSpace(stderr.String()), err
	}
	return strings.TrimSpace(string(out)), nil
}

// sudoDocker runs a docker command
func sudoDocker(args ...string) (string, error) {
	var cmd *exec.Cmd
	if useSudo() {
		cmd = exec.Command("sudo", append([]string{"docker"}, args...)...)
	} else {
		cmd = exec.Command("docker", args...)
	}
	cmd.Stdin = os.Stdin

	var stderr bytes.Buffer
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
	out, err := cmd.Output()
	if err != nil {
		return strings.TrimSpace(stderr.String()), err
	}
	return strings.TrimSpace(string(out)), nil
}
