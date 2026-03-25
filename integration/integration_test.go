//go:build integration

package integration

import (
	"bytes"
	"encoding/binary"
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

// integration tests require Docker and build the full Zeek image.
// run with: go test -tags integration -v -timeout 30m ./integration/...

const testImage = "activecm/zeek:integration-test"

// buildZeekImage builds the Docker image from the repo Dockerfile.
func buildZeekImage(t *testing.T) {
	t.Helper()

	// these must match the values in build.env
	alpineVersion := "3.21"
	zeekVersion := "8.0.6"

	req := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    "..",
				Dockerfile: "Dockerfile",
				Repo:       "activecm/zeek",
				Tag:        "integration-test",
				KeepImage:  true,
				BuildArgs: map[string]*string{
					"ALPINE_VERSION": &alpineVersion,
					"ZEEK_VERSION":   &zeekVersion,
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

	pcapData := buildMinimalPCAP()
	tmpDir := t.TempDir()
	pcapPath := filepath.Join(tmpDir, "test.pcap")
	require.NoError(t, os.WriteFile(pcapPath, pcapData, 0644))

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

	// start without node.cfg - entrypoint should fail
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

	// wait for the healthcheck to pass (start-period is 30s, interval is 60s)
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

	// the init container must override the entrypoint to avoid
	// the node.cfg check. this test verifies that --entrypoint sh works.
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

// helpers

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

// buildMinimalPCAP creates a valid pcap file with a single TCP SYN packet.
// this is enough for zeek to parse and produce a conn.log entry.
func buildMinimalPCAP() []byte {
	var buf bytes.Buffer
	w := &binaryWriter{buf: &buf}

	// pcap global header
	w.u32le(0xa1b2c3d4) // magic
	w.u16le(2)          // version major
	w.u16le(4)          // version minor
	w.i32le(0)          // thiszone
	w.u32le(0)          // sigfigs
	w.u32le(65535)      // snaplen
	w.u32le(1)          // network (LINKTYPE_ETHERNET)

	packet := buildTCPSYNPacket()

	// pcap packet header
	w.u32le(1000000)             // ts_sec
	w.u32le(0)                   // ts_usec
	w.u32le(uint32(len(packet))) // incl_len
	w.u32le(uint32(len(packet))) // orig_len

	buf.Write(packet) //nolint:revive // bytes.Buffer.Write never returns an error
	return buf.Bytes()
}

func buildTCPSYNPacket() []byte {
	var buf bytes.Buffer
	w := &binaryWriter{buf: &buf}

	// ethernet header (14 bytes)
	buf.Write([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}) //nolint:revive // dst mac
	buf.Write([]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}) //nolint:revive // src mac
	w.u16be(0x0800)                                       // ethertype IPv4

	// IPv4 header (20 bytes)
	buf.Write([]byte{0x45, 0x00})  //nolint:revive // version + IHL, DSCP/ECN
	w.u16be(40)                    // total length (20 IP + 20 TCP)
	w.u16be(0x1234)                // identification
	w.u16be(0x4000)                // flags + fragment offset
	buf.Write([]byte{64, 6})       //nolint:revive // TTL, protocol TCP
	w.u16be(0)                     // checksum (zeek doesn't care with -C)
	buf.Write([]byte{10, 0, 0, 1}) //nolint:revive // src IP
	buf.Write([]byte{10, 0, 0, 2}) //nolint:revive // dst IP

	// TCP header (20 bytes)
	w.u16be(12345)  // src port
	w.u16be(80)     // dst port
	w.u32be(1000)   // seq number
	w.u32be(0)      // ack number
	w.u16be(0x5002) // data offset (5) + SYN flag
	w.u16be(65535)  // window
	w.u16be(0)      // checksum
	w.u16be(0)      // urgent pointer

	return buf.Bytes()
}

// binaryWriter wraps binary.Write calls to avoid repetitive error checking.
// bytes.Buffer writes never fail, so errors are intentionally ignored.
type binaryWriter struct {
	buf *bytes.Buffer
}

func (w *binaryWriter) u16le(v uint16) { binary.Write(w.buf, binary.LittleEndian, v) }
func (w *binaryWriter) u32le(v uint32) { binary.Write(w.buf, binary.LittleEndian, v) }
func (w *binaryWriter) i32le(v int32)  { binary.Write(w.buf, binary.LittleEndian, v) }
func (w *binaryWriter) u16be(v uint16) { binary.Write(w.buf, binary.BigEndian, v) }
func (w *binaryWriter) u32be(v uint32) { binary.Write(w.buf, binary.BigEndian, v) }
