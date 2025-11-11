package identity

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// SystemProbe probes system information
type SystemProbe struct{}

// NewSystemProbe creates a new system probe
func NewSystemProbe() *SystemProbe {
	return &SystemProbe{}
}

// ProbeCPUInfo probes CPU information
func (p *SystemProbe) ProbeCPUInfo() (cores int, model string, err error) {
	cores = 1
	model = "unknown"

	// Try to get CPU info from /proc/cpuinfo
	data, err := os.ReadFile("/proc/cpuinfo")
	if err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "processor") {
				cores++
			}
			if strings.HasPrefix(line, "model name") {
				parts := strings.Split(line, ":")
				if len(parts) > 1 {
					model = strings.TrimSpace(parts[1])
				}
			}
		}
	}

	return cores, model, nil
}

// ProbeMemoryInfo probes memory information
func (p *SystemProbe) ProbeMemoryInfo() (totalMB int, availableMB int, err error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "MemTotal:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				kb, _ := strconv.Atoi(parts[1])
				totalMB = kb / 1024
			}
		}
		if strings.HasPrefix(line, "MemAvailable:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				kb, _ := strconv.Atoi(parts[1])
				availableMB = kb / 1024
			}
		}
	}

	return totalMB, availableMB, nil
}

// ProbeDiskInfo probes disk information
func (p *SystemProbe) ProbeDiskInfo() (totalGB int, usedGB int, freeGB int, err error) {
	cmd := exec.Command("df", "-BG", "/")
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, 0, err
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) >= 2 {
		parts := strings.Fields(lines[1])
		if len(parts) >= 4 {
			if size, err := strconv.Atoi(strings.TrimSuffix(parts[1], "G")); err == nil {
				totalGB = size
			}
			if used, err := strconv.Atoi(strings.TrimSuffix(parts[2], "G")); err == nil {
				usedGB = used
			}
			if avail, err := strconv.Atoi(strings.TrimSuffix(parts[3], "G")); err == nil {
				freeGB = avail
			}
		}
	}

	return totalGB, usedGB, freeGB, nil
}

// ProbeKernelInfo probes kernel information
func (p *SystemProbe) ProbeKernelInfo() (version string, release string, err error) {
	cmd := exec.Command("uname", "-r")
	output, err := cmd.Output()
	if err == nil {
		version = strings.TrimSpace(string(output))
	}

	cmd = exec.Command("uname", "-v")
	output, err = cmd.Output()
	if err == nil {
		release = strings.TrimSpace(string(output))
	}

	return version, release, nil
}
