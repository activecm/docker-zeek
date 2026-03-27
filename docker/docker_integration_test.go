//go:build integration

package docker

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// go test -tags integration -v -count=1 ./docker/...

func cleanupContainer(t *testing.T) {
	t.Helper()
	_ = exec.Command("docker", "rm", "--force", ContainerName).Run()
}

func TestInspectNoContainer(t *testing.T) {
	cleanupContainer(t)
	defer cleanupContainer(t)

	state, err := Inspect()
	require.NoError(t, err, "inspect should not error when container doesn't exist")
	require.Nil(t, state, "state should be nil when container doesn't exist")
}

func TestInspectRunningContainer(t *testing.T) {
	cleanupContainer(t)
	defer cleanupContainer(t)

	cmd := exec.Command("docker", "run", "--detach", "--name", ContainerName, "alpine:latest", "sleep", "30")
	require.NoError(t, cmd.Run(), "failed to start test container")

	state, err := Inspect()
	require.NoError(t, err)
	require.NotNil(t, state)
	require.True(t, state.Running)
}

func TestInspectStoppedContainer(t *testing.T) {
	cleanupContainer(t)
	defer cleanupContainer(t)

	cmd := exec.Command("docker", "run", "--detach", "--name", ContainerName, "alpine:latest", "sleep", "30")
	require.NoError(t, cmd.Run(), "failed to start test container")

	cmd = exec.Command("docker", "stop", "-t", "1", ContainerName)
	require.NoError(t, cmd.Run(), "failed to stop test container")

	state, err := Inspect()
	require.NoError(t, err)
	require.NotNil(t, state)
	require.False(t, state.Running)
}

func TestInspectIgnoresSimilarNames(t *testing.T) {
	cleanupContainer(t)
	defer cleanupContainer(t)
	defer func() { _ = exec.Command("docker", "rm", "--force", "zeek-other").Run() }()

	cmd := exec.Command("docker", "run", "--detach", "--name", "zeek-other", "alpine:latest", "sleep", "30")
	require.NoError(t, cmd.Run(), "failed to start decoy container")

	state, err := Inspect()
	require.NoError(t, err)
	require.Nil(t, state, "should not match container with similar name")
}

func TestStopNoContainer(t *testing.T) {
	cleanupContainer(t)

	err := Stop()
	require.NoError(t, err)
}

func TestStatusNoContainer(t *testing.T) {
	cleanupContainer(t)

	err := Status()
	require.NoError(t, err)
}

func TestInitHostDirCreatesDirectories(t *testing.T) {
	cleanupContainer(t)
	defer cleanupContainer(t)

	// use /tmp because Docker Desktop on Mac shares it with the VM
	hostDir := filepath.Join("/tmp", "zeek-init-test-"+t.Name())
	_ = os.RemoveAll(hostDir)
	defer func() { _ = os.RemoveAll(hostDir) }()

	err := InitHostDir("activecm/zeek:integration-test", hostDir)
	require.NoError(t, err)

	for _, dir := range []string{"etc", "logs", "spool", "manual-logs", "share/zeek/site/autoload"} {
		_, err := os.Stat(filepath.Join(hostDir, dir))
		require.NoError(t, err, "expected directory %s to exist", dir)
	}
}

func TestInitHostDirSkipsExistingConfigs(t *testing.T) {
	cleanupContainer(t)
	defer cleanupContainer(t)

	hostDir := filepath.Join("/tmp", "zeek-init-test-"+t.Name())
	_ = os.RemoveAll(hostDir)
	defer func() { _ = os.RemoveAll(hostDir) }()

	err := InitHostDir("activecm/zeek:integration-test", hostDir)
	require.NoError(t, err)

	cfgPath := filepath.Join(hostDir, "etc", "networks.cfg")
	require.NoError(t, os.WriteFile(cfgPath, []byte("custom content"), 0644))

	err = InitHostDir("activecm/zeek:integration-test", hostDir)
	require.NoError(t, err)

	content, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	require.Equal(t, "custom content", string(content))
}
