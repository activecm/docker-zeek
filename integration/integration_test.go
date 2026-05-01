//go:build integration

package integration

import (
	"bytes"
	_ "embed"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// go test -tags integration -v -timeout 30m ./integration/...

const testImage = "activecm/zeek:integration-test"

// minimalPCAP is a DNS-over-UDP capture from Zeek's test suite (github.com/zeek/zeek)
//
//go:embed testdata/dns_original_case.pcap
var minimalPCAP []byte

// buildZeekImage builds the Docker image from the repo Dockerfile
func buildZeekImage(t *testing.T) {
	t.Helper()

	version := "v0.0.0-integration-test"

	req := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    "..",
				Dockerfile: "Dockerfile",
				Repo:       "activecm/zeek",
				Tag:        "integration-test",
				KeepImage:  true,
				BuildArgs: map[string]*string{
					"VERSION": &version,
				},
			},
		},
		Started: false,
	}

	_, err := testcontainers.GenericContainer(t.Context(), req)
	require.NoError(t, err, "failed to build Docker image")
}

func TestImageBuilds(t *testing.T) {
	// skip if image was already built by make docker-build (e.g. in CI)
	out, err := exec.Command("docker", "image", "inspect", testImage).CombinedOutput()
	if err == nil && len(out) > 0 {
		t.Skip("image already exists, skipping build test")
	}
	buildZeekImage(t)
}

func TestZeekStarts(t *testing.T) {
	container := startZeekContainer(t)
	defer terminateContainer(t, container)

	code, output := execInContainer(t, container, "zeekctl", "status")
	require.Equal(t, 0, code, "zeekctl status failed: %s", output)
	require.Contains(t, output, "running")
}

func TestReadPCAP(t *testing.T) {
	ctx := t.Context()

	tmpDir := t.TempDir()
	pcapPath := filepath.Join(tmpDir, "test.pcap")
	require.NoError(t, os.WriteFile(pcapPath, minimalPCAP, 0644))

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: testImage,
			Files: []testcontainers.ContainerFile{
				{
					HostFilePath:      pcapPath,
					ContainerFilePath: "/incoming.pcap",
					FileMode:          0644,
				},
			},
			Entrypoint: []string{"/bin/bash", "-c"},
			Cmd: []string{
				`grep -hv '^#' /usr/local/zeek/share/zeek/site/autoload/*.zeek > /usr/local/zeek/share/zeek/site/local.zeek && ` +
					`(mv -f /usr/local/zeek/share/zeek/builtin-plugins/Zeek_AF_Packet/{__load__.zeek,init.zeek} /usr/local/zeek/share/zeek/builtin-plugins/ 2>/dev/null || true) && ` +
					`cd /usr/local/zeek/logs && /usr/local/zeek/bin/zeek -C -r /incoming.pcap local 'Site::local_nets += { 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16 }' && ls /usr/local/zeek/logs/`,
			},
			WaitingFor: wait.ForExit().WithExitTimeout(120 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err, "failed to start readpcap container")
	defer terminateContainer(t, container)

	logs, err := container.Logs(ctx)
	require.NoError(t, err, "failed to get container logs")
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, logs)
	require.NoError(t, err, "failed to read logs")
	require.Contains(t, buf.String(), "conn.log")
}

func TestCrashRecovery(t *testing.T) {
	container := startZeekContainer(t)
	defer terminateContainer(t, container)

	// kill the zeek process to simulate a crash
	// zeekctl cron only restarts crashed processes, not cleanly stopped ones
	code, _ := execInContainer(t, container, "bash", "-c", "kill -9 $(cat /usr/local/zeek/spool/zeek/.pid)")
	require.Equal(t, 0, code, "failed to kill zeek process")

	_, output := execInContainer(t, container, "zeekctl", "status")
	require.Contains(t, output, "crashed")

	// trigger cron recovery manually
	execInContainer(t, container, "zeekctl", "cron")

	// poll until zeek is running again
	require.Eventually(t, func() bool {
		_, status := execInContainer(t, container, "zeekctl", "status")
		return strings.Contains(status, "running")
	}, 30*time.Second, 2*time.Second, "zeek did not recover after cron")
}

func TestGracefulShutdown(t *testing.T) {
	ctx := t.Context()
	container := startZeekContainer(t)
	defer terminateContainer(t, container)

	state, err := container.State(ctx)
	require.NoError(t, err)
	require.True(t, state.Running, "expected container to be running")

	// stop with a 30s timeout. if the entrypoint handles SIGTERM correctly,
	// zeekctl stop runs and the container exits well before the timeout.
	// if SIGTERM is ignored, Docker waits the full timeout then sends SIGKILL.
	timeout := 30 * time.Second
	start := time.Now()
	require.NoError(t, container.Stop(ctx, &timeout))
	elapsed := time.Since(start)

	state, err = container.State(ctx)
	require.NoError(t, err)
	require.False(t, state.Running, "expected container to be stopped")

	// a shutdown via zeekctl stop should complete in < 15 seconds.
	// if it took close to the full timeout, SIGTERM was probably ignored
	require.Less(t, elapsed, 20*time.Second, "container took too long to stop, SIGTERM may not be handled")
}

func TestLogOutput(t *testing.T) {
	container := startZeekContainer(t)
	defer terminateContainer(t, container)

	// give zeek a few seconds to write initial logs
	time.Sleep(10 * time.Second)

	_, output := execInContainer(t, container, "ls", "/usr/local/zeek/spool/zeek/")
	require.Contains(t, output, "loaded_scripts.log")
}

func TestEntrypointValidation(t *testing.T) {
	ctx := t.Context()

	// start without node.cfg, entrypoint should fail
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:      testImage,
			WaitingFor: wait.ForExit().WithExitTimeout(30 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err, "failed to start container")
	defer terminateContainer(t, container)

	state, err := container.State(ctx)
	require.NoError(t, err)
	require.NotEqual(t, 0, state.ExitCode, "expected non-zero exit code when node.cfg is missing")
}

func TestHealthcheck(t *testing.T) {
	ctx := t.Context()
	container := startZeekContainer(t)
	defer terminateContainer(t, container)

	// wait for the healthcheck to pass
	require.Eventually(t, func() bool {
		state, err := container.State(ctx)
		if err != nil {
			return false
		}
		return state.Health != nil && state.Health.Status == "healthy"
	}, 120*time.Second, 5*time.Second, "container never became healthy")
}

func TestInitContainerOverridesEntrypoint(t *testing.T) {
	ctx := t.Context()

	// the init container has to override the entrypoint to avoid the node.cfg check
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:      testImage,
			Entrypoint: []string{"sh"},
			Cmd:        []string{"-c", "echo init-ok && ls /usr/local/zeek/etc/"},
			WaitingFor: wait.ForExit().WithExitTimeout(30 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err, "failed to start init container")
	defer terminateContainer(t, container)

	state, err := container.State(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, state.ExitCode, "init container should exit 0 when entrypoint is overridden")

	logs, err := container.Logs(ctx)
	require.NoError(t, err)
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, logs)
	require.NoError(t, err)
	require.Contains(t, buf.String(), "init-ok")
}

func TestBakedPackagesPresent(t *testing.T) {
	ctx := t.Context()

	// verify the baked in packages show up in zkg list
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:      testImage,
			Entrypoint: []string{"sh"},
			Cmd:        []string{"-c", "zkg list"},
			WaitingFor: wait.ForExit().WithExitTimeout(30 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err, "failed to start container")
	defer terminateContainer(t, container)

	state, err := container.State(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, state.ExitCode, "zkg list should exit 0")

	logs, err := container.Logs(ctx)
	require.NoError(t, err)
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, logs)
	require.NoError(t, err)
	output := buf.String()

	require.Contains(t, output, "ja3")
	require.Contains(t, output, "ja4")
	require.Contains(t, output, "zeek-open-connections")
}

// startZeekContainer starts a zeek container in standalone mode on loopback
func startZeekContainer(t *testing.T) testcontainers.Container {
	t.Helper()
	ctx := t.Context()

	nodeCfg := `[zeek]
type=standalone
host=localhost
interface=lo
`
	tmpDir := t.TempDir()
	nodeCfgPath := filepath.Join(tmpDir, "node.cfg")
	require.NoError(t, os.WriteFile(nodeCfgPath, []byte(nodeCfg), 0644))

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: testImage,
			Files: []testcontainers.ContainerFile{
				{
					HostFilePath:      nodeCfgPath,
					ContainerFilePath: "/usr/local/zeek/etc/node.cfg",
					FileMode:          0644,
				},
			},
			HostConfigModifier: func(hc *dockercontainer.HostConfig) {
				hc.CapAdd = []string{"NET_RAW", "NET_ADMIN"}
				hc.NetworkMode = "host"
			},
			WaitingFor: wait.ForLog("cron enabled").WithStartupTimeout(120 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err, "failed to start zeek container")
	return container
}

func terminateContainer(t *testing.T, container testcontainers.Container) {
	t.Helper()
	if err := container.Terminate(t.Context()); err != nil {
		t.Logf("failed to terminate container: %v", err)
	}
}

func execInContainer(t *testing.T, container testcontainers.Container, cmd ...string) (int, string) {
	t.Helper()
	code, output, err := container.Exec(t.Context(), cmd)
	require.NoError(t, err, "failed to exec %v", cmd)
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, output)
	require.NoError(t, err, "failed to read exec output")
	return code, buf.String()
}
