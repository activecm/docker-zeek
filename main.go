package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/activecm/docker-zeek/docker"
	"github.com/activecm/docker-zeek/sensor"
	"github.com/urfave/cli/v3"
)

const defaultHostDir = "/opt/zeek"

// Version is populated by build flags or defaults to "dev"
var Version string

// DefaultRelease is the Docker image tag this CLI was built for.
// populated at build time via -ldflags: from build.env locally, from the git tag in CI.
// falls back to "latest" when unset (e.g. "go run .")
var DefaultRelease string

func main() {
	if Version == "" {
		Version = "dev"
	}

	app := &cli.Command{
		Name:    "zeek",
		Usage:   "manage a Zeek Docker container",
		Version: Version,
		Commands: []*cli.Command{
			{
				Name:  "start",
				Usage: "start the Zeek container",
				Action: func(_ context.Context, _ *cli.Command) error {
					image, hostDir := resolveConfig()
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
					image, hostDir := resolveConfig()
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
					image, hostDir := resolveConfig()
					return readpcap(cmd, image, hostDir)
				},
			},
			{
				Name:  "update",
				Usage: "pull the latest image and restart",
				Action: func(_ context.Context, _ *cli.Command) error {
					image, hostDir := resolveConfig()
					fmt.Fprintln(os.Stderr, "Pulling latest Zeek image")
					if err := docker.Pull(image); err != nil {
						return err
					}
					if err := docker.Stop(); err != nil {
						return err
					}
					return start(image, hostDir)
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

// envWithFallback checks the environment for an uppercase variable first then checks lowercase for backwards compatibility
// with the legacy version which used lowercase.
func envWithFallback(upper, lower string) string {
	if v := os.Getenv(upper); v != "" {
		return v
	}
	return os.Getenv(lower)
}

func resolveConfig() (string, string) {
	hostDir := envWithFallback("ZEEK_TOP_DIR", "zeek_top_dir")
	if hostDir == "" {
		hostDir = defaultHostDir
	}

	release := envWithFallback("ZEEK_RELEASE", "zeek_release")
	if release == "" {
		if DefaultRelease != "" {
			release = DefaultRelease
		} else {
			release = "latest"
		}
	}
	image := docker.DefaultImage + ":" + release
	return image, hostDir
}

func start(image, hostDir string) error {
	if err := docker.ValidatePath(hostDir); err != nil {
		return err
	}
	if err := docker.InitHostDir(image, hostDir); err != nil {
		return err
	}
	if err := checkWriteAccess(filepath.Join(hostDir, "etc")); err != nil {
		return err
	}

	nodeCfgPath := filepath.Join(hostDir, "etc", "node.cfg")
	if err := ensureNodeCfg(nodeCfgPath); err != nil {
		return err
	}

	return docker.Start(image, hostDir)
}

// checkWriteAccess verifies the current user can write to a directory.
// returns a clear error message suggesting sudo if permission is denied.
func checkWriteAccess(dir string) error {
	tmp := filepath.Join(dir, ".write-test")
	f, err := os.Create(tmp)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("cannot write to %s - run with sudo or ensure your user has write access", dir)
		}
		return err
	}
	_ = f.Close()
	_ = os.Remove(tmp)
	return nil
}

func ensureNodeCfg(path string) error {
	info, err := os.Stat(path)
	if err == nil && info.Size() > 0 {
		return nil
	}

	fmt.Fprintln(os.Stderr, "No node.cfg found. Starting sensor setup.")
	reader := bufio.NewReader(os.Stdin)
	cfg, err := sensor.PromptForConfig(reader)
	if err != nil {
		return fmt.Errorf("sensor setup: %w", err)
	}

	return sensor.GenerateNodeCfg(cfg, path)
}

func readpcap(cmd *cli.Command, image, hostDir string) error {
	args := cmd.Args()
	if args.Len() < 1 {
		return errors.New("readpcap requires a pcap file path")
	}

	pcapPath := args.Get(0)
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
		logDir = args.Get(1)
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

	if err := docker.InitHostDir(image, hostDir); err != nil {
		return err
	}

	return docker.ReadPCAP(image, hostDir, pcapPath, logDir)
}
