package identity

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// isContainer checks if we're running in a container
func isContainer() bool {
	// Check for common container indicators
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	// Check cgroup v1
	if data, err := os.ReadFile("/proc/self/cgroup"); err == nil {
		content := string(data)
		if strings.Contains(content, "docker") || strings.Contains(content, "containerd") || strings.Contains(content, "kubepods") {
			return true
		}
	}
	return false
}

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

	// CPU cores - use container limits if in container
	if cpuCores, err := c.getCPUCores(); err == nil {
		metadata.CPUCores = cpuCores
	} else {
		metadata.CPUCores = runtime.NumCPU()
	}

	// Memory - use container limits if in container
	if mem, err := c.getMemoryMB(); err == nil {
		metadata.MemoryMB = mem
	}

	// Disk (root filesystem) - container filesystem
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

func (c *Collector) getCPUCores() (int, error) {
	if !isContainer() {
		return 0, fmt.Errorf("not in container")
	}

	// Try cgroup v2 first
	if quota, err := c.readCgroupV2CPU(); err == nil {
		return quota, nil
	}

	// Try cgroup v1
	if quota, err := c.readCgroupV1CPU(); err == nil {
		return quota, nil
	}

	return 0, fmt.Errorf("could not determine CPU cores from cgroup")
}

func (c *Collector) readCgroupV2CPU() (int, error) {
	// Read from /sys/fs/cgroup/cpu.max (format: "quota period" or "max")
	data, err := os.ReadFile("/sys/fs/cgroup/cpu.max")
	if err != nil {
		return 0, err
	}

	content := strings.TrimSpace(string(data))
	if content == "max" {
		// No limit, fall back to runtime
		return runtime.NumCPU(), nil
	}

	parts := strings.Fields(content)
	if len(parts) >= 2 {
		quota, err1 := strconv.ParseInt(parts[0], 10, 64)
		period, err2 := strconv.ParseInt(parts[1], 10, 64)
		if err1 == nil && err2 == nil && period > 0 {
			cores := float64(quota) / float64(period)
			if cores > 0 {
				return int(cores + 0.5), nil // Round up
			}
		}
	}

	return 0, fmt.Errorf("invalid cpu.max format")
}

func (c *Collector) readCgroupV1CPU() (int, error) {
	// Try to find the cgroup path
	cgroupPaths := []string{
		"/sys/fs/cgroup/cpu/cpu.cfs_quota_us",
		"/sys/fs/cgroup/cpu,cpuacct/cpu.cfs_quota_us",
	}

	var quotaData, periodData []byte
	var err error

	for _, basePath := range cgroupPaths {
		quotaPath := basePath
		periodPath := strings.Replace(basePath, "cpu.cfs_quota_us", "cpu.cfs_period_us", 1)

		quotaData, err = os.ReadFile(quotaPath)
		if err != nil {
			continue
		}

		periodData, err = os.ReadFile(periodPath)
		if err != nil {
			continue
		}

		break
	}

	if err != nil || quotaData == nil || periodData == nil {
		return 0, fmt.Errorf("could not read cgroup v1 cpu files")
	}

	quota, err1 := strconv.ParseInt(strings.TrimSpace(string(quotaData)), 10, 64)
	period, err2 := strconv.ParseInt(strings.TrimSpace(string(periodData)), 10, 64)

	if err1 != nil || err2 != nil || period <= 0 {
		return 0, fmt.Errorf("invalid cgroup cpu values")
	}

	// -1 means no limit
	if quota == -1 {
		return runtime.NumCPU(), nil
	}

	cores := float64(quota) / float64(period)
	if cores > 0 {
		return int(cores + 0.5), nil // Round up
	}

	return 0, fmt.Errorf("could not calculate CPU cores")
}

func (c *Collector) getMemoryMB() (int, error) {
	if runtime.GOOS != "linux" {
		return 0, fmt.Errorf("not linux")
	}

	// If in container, try to get container memory limit first
	if isContainer() {
		// Try cgroup v2 first
		if mem, err := c.readCgroupV2Memory(); err == nil {
			return mem, nil
		}

		// Try cgroup v1
		if mem, err := c.readCgroupV1Memory(); err == nil {
			return mem, nil
		}
	}

	// Fallback to /proc/meminfo (host memory if not in container, or if cgroup read fails)
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
	return 0, fmt.Errorf("could not determine memory")
}

func (c *Collector) readCgroupV2Memory() (int, error) {
	// Read from /sys/fs/cgroup/memory.max
	data, err := os.ReadFile("/sys/fs/cgroup/memory.max")
	if err != nil {
		return 0, err
	}

	content := strings.TrimSpace(string(data))
	if content == "max" {
		// No limit, fall back to /proc/meminfo
		return 0, fmt.Errorf("no memory limit")
	}

	// Value is in bytes
	bytes, err := strconv.ParseInt(content, 10, 64)
	if err != nil {
		return 0, err
	}

	// Convert to MB
	return int(bytes / 1024 / 1024), nil
}

func (c *Collector) readCgroupV1Memory() (int, error) {
	// Try to find the cgroup path
	cgroupPaths := []string{
		"/sys/fs/cgroup/memory/memory.limit_in_bytes",
		"/sys/fs/cgroup/memory,cpuacct/memory.limit_in_bytes",
	}

	var data []byte
	var err error

	for _, path := range cgroupPaths {
		data, err = os.ReadFile(path)
		if err == nil {
			break
		}
	}

	if err != nil || data == nil {
		return 0, fmt.Errorf("could not read cgroup v1 memory file")
	}

	content := strings.TrimSpace(string(data))
	// 9223372036854771712 is typically "max" in cgroup v1
	if content == "9223372036854771712" || content == "max" {
		return 0, fmt.Errorf("no memory limit")
	}

	// Value is in bytes
	bytes, err := strconv.ParseInt(content, 10, 64)
	if err != nil {
		return 0, err
	}

	// Convert to MB
	return int(bytes / 1024 / 1024), nil
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
