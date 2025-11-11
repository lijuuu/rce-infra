package identity

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// Metadata represents system metadata
type Metadata struct {
	OSName        string `json:"os_name,omitempty"`
	OSVersion     string `json:"os_version,omitempty"`
	Arch          string `json:"arch,omitempty"`
	KernelVersion string `json:"kernel_version,omitempty"`
	Hostname      string `json:"hostname,omitempty"`
	IPAddress     string `json:"ip_address,omitempty"`
	CPUCores      int    `json:"cpu_cores,omitempty"`
	MemoryMB      int    `json:"memory_mb,omitempty"`
	DiskGB        int    `json:"disk_gb,omitempty"`
}

// Collector collects system metadata
type Collector struct{}

// NewCollector creates a new metadata collector
func NewCollector() *Collector {
	return &Collector{}
}

// Collect collects system metadata
func (c *Collector) Collect() (*Metadata, error) {
	metadata := &Metadata{
		Arch: runtime.GOARCH,
	}

	// Hostname
	if hostname, err := os.Hostname(); err == nil {
		metadata.Hostname = hostname
	}

	// OS info
	if runtime.GOOS == "linux" {
		metadata.OSName = "linux"
		if osRelease, err := c.readOSRelease(); err == nil {
			metadata.OSVersion = osRelease
		}
		if kernel, err := c.execCommand("uname", "-r"); err == nil {
			metadata.KernelVersion = strings.TrimSpace(kernel)
		}
	} else {
		metadata.OSName = runtime.GOOS
	}

	// IP address (first non-loopback interface)
	if ip, err := c.getIPAddress(); err == nil {
		metadata.IPAddress = ip
	}

	// CPU cores
	metadata.CPUCores = runtime.NumCPU()

	// Memory (approximate)
	if mem, err := c.getMemoryMB(); err == nil {
		metadata.MemoryMB = mem
	}

	// Disk (root filesystem)
	if disk, err := c.getDiskGB(); err == nil {
		metadata.DiskGB = disk
	}

	return metadata, nil
}

func (c *Collector) readOSRelease() (string, error) {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\""), nil
		}
	}

	return "", fmt.Errorf("PRETTY_NAME not found")
}

func (c *Collector) execCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func (c *Collector) getIPAddress() (string, error) {
	// Try to get IP from ip command
	if ip, err := c.execCommand("ip", "route", "get", "1"); err == nil {
		parts := strings.Fields(ip)
		for i, part := range parts {
			if part == "src" && i+1 < len(parts) {
				return parts[i+1], nil
			}
		}
	}

	// Fallback to hostname -I
	if ip, err := c.execCommand("hostname", "-I"); err == nil {
		ips := strings.Fields(ip)
		if len(ips) > 0 {
			return ips[0], nil
		}
	}

	return "", fmt.Errorf("could not determine IP address")
}

func (c *Collector) getMemoryMB() (int, error) {
	if runtime.GOOS == "linux" {
		data, err := os.ReadFile("/proc/meminfo")
		if err != nil {
			return 0, err
		}

		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "MemTotal:") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					kb, err := strconv.Atoi(parts[1])
					if err == nil {
						return kb / 1024, nil
					}
				}
			}
		}
	}
	return 0, fmt.Errorf("could not determine memory")
}

func (c *Collector) getDiskGB() (int, error) {
	if runtime.GOOS == "linux" {
		output, err := c.execCommand("df", "-BG", "/")
		if err != nil {
			return 0, err
		}

		lines := strings.Split(string(output), "\n")
		if len(lines) >= 2 {
			parts := strings.Fields(lines[1])
			if len(parts) >= 2 {
				sizeStr := strings.TrimSuffix(parts[1], "G")
				if size, err := strconv.Atoi(sizeStr); err == nil {
					return size, nil
				}
			}
		}
	}
	return 0, fmt.Errorf("could not determine disk size")
}
