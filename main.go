package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/activecm/docker-zeek/docker"
	"github.com/activecm/docker-zeek/sensor"
	"github.com/urfave/cli/v3"
)

const defaultHostDir = "/opt/zeek"

// Version is the docker-zeek version
var Version string

// ImageTag is the docker image tag
var ImageTag string

var ErrNotBuilt = errors.New("docker-zeek not built properly: run `make build`")

func main() {
	app := &cli.Command{
		Name:    "zeek",
		Usage:   "manage a Zeek Docker container",
		Version: Version,
		Commands: []*cli.Command{
			{
				Name:  "start",
				Usage: "start the Zeek container",
				Action: func(_ context.Context, _ *cli.Command) error {
					image, hostDir, err := resolveConfig()
					if err != nil {
						return err
					}
					return start(image, hostDir)
				},
			},
			{
				Name:  "stop",
				Usage: "stop the Zeek container",
				Action: func(_ context.Context, _ *cli.Command) error {
					return docker.Stop()
				},
			},
			{
				Name:  "restart",
				Usage: "restart the Zeek container",
				Action: func(_ context.Context, _ *cli.Command) error {
					image, hostDir, err := resolveConfig()
					if err != nil {
						return err
					}
					if err := docker.Stop(); err != nil {
						return err
					}
					return start(image, hostDir)
				},
			},
			{
				Name:  "status",
				Usage: "show Zeek container and process status",
				Action: func(_ context.Context, _ *cli.Command) error {
					return docker.Status()
				},
			},
			{
				Name:      "readpcap",
				Usage:     "process a pcap file with Zeek",
				ArgsUsage: "<pcap-file> [output-dir]",
				Action: func(_ context.Context, cmd *cli.Command) error {
					image, hostDir, err := resolveConfig()
					if err != nil {
						return err
					}
					return readpcap(cmd, image, hostDir)
				},
			},
		},
		CommandNotFound: func(_ context.Context, _ *cli.Command, s string) {
			fmt.Fprintf(os.Stderr, "unknown command: %s\n", s)
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func resolveConfig() (string, string, error) {
	if Version == "" || ImageTag == "" {
		return "", "", ErrNotBuilt
	}
	hostDir := os.Getenv("ZEEK_TOP_DIR")
	if hostDir == "" {
		hostDir = os.Getenv("zeek_top_dir")
	}
	if hostDir == "" {
		hostDir = defaultHostDir
	}
	image := docker.DefaultImage + ":" + ImageTag
	return image, hostDir, nil
}

func start(image, hostDir string) error {
	// exit if zeek is already running before doing any setup
	state, err := docker.Inspect()
	if err != nil {
		return err
	}
	if state != nil && state.Running {
		if state.Image != image {
			return fmt.Errorf("zeek is running on %s, not %s. Run 'zeek stop' to remove it, then 'zeek start' to launch this version", state.Image, image)
		}
		fmt.Fprintln(os.Stderr, "Zeek is already running.")
		return nil
	}

	if err := docker.ValidatePath(hostDir); err != nil {
		return err
	}
	if err := docker.InitHostDir(image, hostDir, Version); err != nil {
		return err
	}
	if err := checkWriteAccess(filepath.Join(hostDir, "etc")); err != nil {
		return err
	}

	envPath := filepath.Join(hostDir, "etc", "zeek.env")
	envVars, err := readZeekEnv(envPath)
	if err != nil {
		return fmt.Errorf("reading zeek.env: %w", err)
	}

	nodeCfgPath := filepath.Join(hostDir, "etc", "node.cfg")
	_, nodeCfgErr := os.Stat(nodeCfgPath)
	nodeCfgExists := nodeCfgErr == nil

	// run sensor setup if no mounted node.cfg and no ZEEK_INTERFACE in zeek.env
	if !nodeCfgExists && envVars["ZEEK_INTERFACE"] == "" {
		fmt.Fprintln(os.Stderr, "Starting sensor setup.")
		reader := bufio.NewReader(os.Stdin)
		names, err := sensor.InterfaceSelectionPrompt(reader)
		if err != nil {
			return fmt.Errorf("sensor setup: %w", err)
		}
		envVars["ZEEK_INTERFACE"] = strings.Join(names, ",")
		if err := writeZeekEnv(envPath, envVars); err != nil {
			return fmt.Errorf("writing zeek.env: %w", err)
		}
	}

	docker.WarnLegacyVolumes()

	return docker.Start(image, hostDir, envVars)
}

// checkWriteAccess verifies write access to a directory by creating a probe file
func checkWriteAccess(dir string) error {
	tmp := filepath.Join(dir, ".write-test")
	f, err := os.Create(tmp)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("cannot write to %s - run with sudo or ensure your user has write access", dir)
		}
		return err
	}
	f.Close()
	os.Remove(tmp)
	return nil
}

// readZeekEnv parses a zeek.env file and returns its key-value pairs
func readZeekEnv(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	defer f.Close()

	vals := map[string]string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			vals[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return vals, scanner.Err()
}

// writeZeekEnv writes the given key-value pairs to a zeek.env file
func writeZeekEnv(path string, vals map[string]string) error {
	var b strings.Builder
	for k, v := range vals {
		fmt.Fprintf(&b, "%s=%s\n", k, v)
	}
	return os.WriteFile(path, []byte(b.String()), 0644)
}

func readpcap(cmd *cli.Command, image, hostDir string) error {
	args := cmd.Args()
	if args.Len() < 1 {
		return errors.New("readpcap requires a pcap file path")
	}

	// resolve relative CLI paths before validating
	pcapPath, err := filepath.Abs(args.Get(0))
	if err != nil {
		return fmt.Errorf("resolving pcap path: %w", err)
	}

	info, err := os.Stat(pcapPath)
	if err != nil {
		return fmt.Errorf("pcap file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("pcap file path is a directory: %s", pcapPath)
	}

	for _, p := range []string{hostDir, pcapPath} {
		if err := docker.ValidatePath(p); err != nil {
			return err
		}
	}

	logDir := filepath.Join(hostDir, "manual-logs")
	if args.Len() >= 2 {
		logDir, err = filepath.Abs(args.Get(1))
		if err != nil {
			return fmt.Errorf("resolving log directory: %w", err)
		}
		if err := docker.ValidatePath(logDir); err != nil {
			return err
		}
	}

	if err := os.MkdirAll(logDir, 0755); err != nil {
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("cannot write to %s - run with sudo or ensure your user has write access", logDir)
		}
		return fmt.Errorf("creating log directory: %w", err)
	}

	if err := docker.InitHostDir(image, hostDir, Version); err != nil {
		return err
	}

	return docker.ReadPCAP(image, hostDir, pcapPath, logDir)
}
