package utils

import (
	"strings"
)

// NormalizeOSName normalizes OS name for consistent storage
func NormalizeOSName(osName string) string {
	osName = strings.ToLower(osName)
	if strings.Contains(osName, "ubuntu") {
		return "ubuntu"
	}
	if strings.Contains(osName, "debian") {
		return "debian"
	}
	if strings.Contains(osName, "centos") {
		return "centos"
	}
	if strings.Contains(osName, "rhel") || strings.Contains(osName, "redhat") {
		return "rhel"
	}
	if strings.Contains(osName, "fedora") {
		return "fedora"
	}
	if strings.Contains(osName, "amazon") || strings.Contains(osName, "amzn") {
		return "amazon-linux"
	}
	return osName
}

// NormalizeArch normalizes architecture name
func NormalizeArch(arch string) string {
	arch = strings.ToLower(arch)
	switch arch {
	case "x86_64", "amd64":
		return "amd64"
	case "aarch64", "arm64":
		return "arm64"
	case "i386", "i686", "x86":
		return "i386"
	default:
		return arch
	}
}

// DiffMetadata compares two metadata maps and returns differences
func DiffMetadata(old, new map[string]interface{}) map[string]interface{} {
	diff := make(map[string]interface{})

	for key, newVal := range new {
		oldVal, exists := old[key]
		if !exists || oldVal != newVal {
			diff[key] = newVal
		}
	}

	return diff
}
