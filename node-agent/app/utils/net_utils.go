package utils

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
)

// GetPrimaryIP returns the primary IP address of the node
func GetPrimaryIP() (string, error) {
	// Try to get IP from ip command
	cmd := exec.Command("ip", "route", "get", "1")
	output, err := cmd.Output()
	if err == nil {
		parts := strings.Fields(string(output))
		for i, part := range parts {
			if part == "src" && i+1 < len(parts) {
				return parts[i+1], nil
			}
		}
	}

	// Fallback to hostname -I
	cmd = exec.Command("hostname", "-I")
	output, err = cmd.Output()
	if err == nil {
		ips := strings.Fields(string(output))
		if len(ips) > 0 {
			return ips[0], nil
		}
	}

	// Fallback to interface enumeration
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("failed to get interfaces: %w", err)
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					return ipnet.IP.String(), nil
				}
			}
		}
	}

	return "", fmt.Errorf("could not determine IP address")
}

// GetNetworkInterfaces returns all network interfaces
func GetNetworkInterfaces() ([]net.Interface, error) {
	return net.Interfaces()
}

// IsReachable checks if a host is reachable
func IsReachable(host string, port string) bool {
	conn, err := net.Dial("tcp", net.JoinHostPort(host, port))
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
