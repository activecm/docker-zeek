package sensor

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

type InterfaceInfo struct {
	Name string
	Up   bool
	IP   string
}

// ListInterfaces returns all non-loopback network interfaces with their state and IP
func ListInterfaces() ([]InterfaceInfo, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("listing interfaces: %w", err)
	}
	var result []InterfaceInfo
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		info := InterfaceInfo{
			Name: iface.Name,
			Up:   iface.Flags&net.FlagUp != 0,
		}
		var v6 string
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			ip, _, _ := net.ParseCIDR(addr.String())
			if ip == nil {
				continue
			}
			if ip.To4() != nil {
				if info.IP == "" {
					info.IP = ip.String()
				}
			} else if v6 == "" {
				v6 = ip.String()
			}
		}
		if info.IP == "" {
			info.IP = v6
		}
		result = append(result, info)
	}
	return result, nil
}

// InterfaceSelectionPrompt prompts the user to select capture interfaces
func InterfaceSelectionPrompt(reader *bufio.Reader) ([]string, error) {
	ifaces, err := ListInterfaces()
	if err != nil {
		return nil, err
	}
	if len(ifaces) == 0 {
		return nil, errors.New("no network interfaces found")
	}

	fmt.Fprintln(os.Stderr, "Available network interfaces (* = recommended):")
	for i, iface := range ifaces {
		marker := " "
		if iface.IsRecommended() {
			marker = "*"
		}
		state := "DOWN"
		if iface.Up {
			state = "UP"
		}
		fmt.Fprintf(os.Stderr, "  %s %d) %-12s %-4s %s\n", marker, i+1, iface.Name, state, iface.IP)
	}

	selected, err := getUserSelections(reader, "Select interface(s) (e.g. 1,3)", len(ifaces))
	if err != nil {
		return nil, err
	}

	names := make([]string, len(selected))
	for i, n := range selected {
		names[i] = ifaces[n-1].Name
	}
	return names, nil
}

func getUserSelections(reader *bufio.Reader, prompt string, count int) ([]int, error) {
	for {
		fmt.Fprintf(os.Stderr, "%s: ", prompt)
		input, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("reading input: %w", err)
		}
		var nums []int
		valid := true
		for s := range strings.SplitSeq(strings.TrimSpace(input), ",") {
			n, err := strconv.Atoi(strings.TrimSpace(s))
			if err != nil || n < 1 || n > count {
				fmt.Fprintf(os.Stderr, "Please pick numbers between 1 and %d.\n", count)
				valid = false
				break
			}
			nums = append(nums, n)
		}
		if !valid {
			continue
		}
		if len(nums) == 0 {
			fmt.Fprintln(os.Stderr, "Please select at least one.")
			continue
		}
		return nums, nil
	}
}

// IsRecommended checks whether this interface is a likely capture target
func (i InterfaceInfo) IsRecommended() bool {
	if !i.Up || i.IP != "" {
		return false
	}
	for _, prefix := range []string{"br-", "veth", "virb", "docker", "wlan", "wlp", "wlx"} {
		if strings.HasPrefix(i.Name, prefix) {
			return false
		}
	}
	return true
}
