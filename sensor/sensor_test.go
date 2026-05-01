package sensor

import (
	"bufio"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListInterfaces(t *testing.T) {
	t.Run("Excludes Loopback", func(t *testing.T) {
		ifaces, err := ListInterfaces()
		require.NoError(t, err)
		for _, i := range ifaces {
			require.NotEqual(t, "lo", i.Name)
		}
	})
}

func TestIsRecommended(t *testing.T) {
	t.Run("Recommends Up", func(t *testing.T) {
		i := InterfaceInfo{Name: "eth1", Up: true}
		require.True(t, i.IsRecommended())
	})

	t.Run("Excludes Down", func(t *testing.T) {
		i := InterfaceInfo{Name: "eth1", Up: false}
		require.False(t, i.IsRecommended())
	})

	t.Run("Excludes Has IP", func(t *testing.T) {
		i := InterfaceInfo{Name: "eth0", Up: true, IP: "192.168.1.1"}
		require.False(t, i.IsRecommended())
	})

	t.Run("Excludes Virtual Prefixes", func(t *testing.T) {
		for _, name := range []string{"br-abc", "veth123", "virbr0", "docker0", "wlan0", "wlp1s0", "wlx00"} {
			i := InterfaceInfo{Name: name, Up: true}
			require.False(t, i.IsRecommended(), "expected %s to not be recommended", name)
		}
	})
}

func TestPromptMultiSelection(t *testing.T) {
	t.Run("Single Number", func(t *testing.T) {
		r := bufio.NewReader(strings.NewReader("2\n"))
		nums, err := getUserSelections(r, "pick", 5)
		require.NoError(t, err)
		require.Equal(t, []int{2}, nums)
	})

	t.Run("Multiple Comma Separated", func(t *testing.T) {
		r := bufio.NewReader(strings.NewReader("1,3,5\n"))
		nums, err := getUserSelections(r, "pick", 5)
		require.NoError(t, err)
		require.Equal(t, []int{1, 3, 5}, nums)
	})

	t.Run("Trims Whitespace", func(t *testing.T) {
		r := bufio.NewReader(strings.NewReader("  1 , 3 \n"))
		nums, err := getUserSelections(r, "pick", 5)
		require.NoError(t, err)
		require.Equal(t, []int{1, 3}, nums)
	})

	t.Run("Out Of Range", func(t *testing.T) {
		r := bufio.NewReader(strings.NewReader("0\n6\n2\n"))
		nums, err := getUserSelections(r, "pick", 5)
		require.NoError(t, err)
		require.Equal(t, []int{2}, nums)
	})

	t.Run("Non-Numeric", func(t *testing.T) {
		r := bufio.NewReader(strings.NewReader("foo\n2\n"))
		nums, err := getUserSelections(r, "pick", 5)
		require.NoError(t, err)
		require.Equal(t, []int{2}, nums)
	})

	t.Run("Empty Line", func(t *testing.T) {
		r := bufio.NewReader(strings.NewReader("\n2\n"))
		nums, err := getUserSelections(r, "pick", 5)
		require.NoError(t, err)
		require.Equal(t, []int{2}, nums)
	})

	t.Run("Empty Reader", func(t *testing.T) {
		r := bufio.NewReader(strings.NewReader(""))
		_, err := getUserSelections(r, "pick", 5)
		require.ErrorIs(t, err, io.EOF)
	})
}
