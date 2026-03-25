package sensor

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"text/template"
)

const nodeCfgTemplate = `[manager]
type=manager
host=localhost

[proxy-1]
type=proxy
host=localhost

[worker-1]
type=worker
host=localhost
interface=af_packet::{{.Interface}}
lb_method=custom
lb_procs={{.Workers}}
af_packet_fanout_id=23
af_packet_fanout_mode=AF_Packet::FANOUT_HASH
af_packet_buffer_size=128*1024*1024
`

type NodeConfig struct {
	Interface string
	Workers   int
}

// ListInterfaces returns the names of all (non-loopback) network interfaces that are up on the host machine
func ListInterfaces() ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("listing interfaces: %w", err)
	}

	var names []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		names = append(names, iface.Name)
	}
	return names, nil
}

// PromptForConfig interactively asks the user to select an interface and number of worker processes
func PromptForConfig(reader *bufio.Reader) (*NodeConfig, error) {
	ifaces, err := ListInterfaces()
	if err != nil {
		return nil, err
	}
	if len(ifaces) == 0 {
		return nil, errors.New("no suitable network interfaces found")
	}

	fmt.Fprintln(os.Stderr, "Available network interfaces:")
	for i, name := range ifaces {
		fmt.Fprintf(os.Stderr, "  %d) %s\n", i+1, name)
	}

	iface, err := promptSelection(reader, "Select an interface", len(ifaces))
	if err != nil {
		return nil, err
	}

	fmt.Fprint(os.Stderr, "Number of worker processes (0 for auto): ")
	workersStr, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("reading worker count: %w", err)
	}
	workersStr = strings.TrimSpace(workersStr)

	workers, err := strconv.Atoi(workersStr)
	if err != nil || workers < 0 {
		return nil, fmt.Errorf("invalid worker count: %s", workersStr)
	}

	if workers == 0 {
		workers = AutoWorkerCount()
	}

	return &NodeConfig{
		Interface: ifaces[iface-1],
		Workers:   workers,
	}, nil
}

// GenerateNodeCfg writes a node.cfg file
func GenerateNodeCfg(cfg *NodeConfig, path string) error {
	content, err := RenderNodeCfg(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0640)
}

// RenderNodeCfg returns the rendered node.cfg content as a string
func RenderNodeCfg(cfg *NodeConfig) (string, error) {
	tmpl, err := template.New("node.cfg").Parse(nodeCfgTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return "", fmt.Errorf("rendering template: %w", err)
	}
	return buf.String(), nil
}

// AutoWorkerCount returns CPUs minus 1 (minimum 1)
func AutoWorkerCount() int {
	return max(runtime.NumCPU()-1, 1)
}

func promptSelection(reader *bufio.Reader, prompt string, count int) (int, error) {
	for {
		fmt.Fprintf(os.Stderr, "%s [1-%d]: ", prompt, count)
		input, err := reader.ReadString('\n')
		if err != nil {
			return 0, fmt.Errorf("reading input: %w", err)
		}
		input = strings.TrimSpace(input)
		n, err := strconv.Atoi(input)
		if err != nil || n < 1 || n > count {
			fmt.Fprintf(os.Stderr, "Please enter a number between 1 and %d.\n", count)
			continue
		}
		return n, nil
	}
}
