//go:build darwin || linux

package platform

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// InterfaceInfo holds the human-readable name and type for a network interface.
type InterfaceInfo struct {
	HardwarePort string // e.g. "Wi-Fi", "USB 10/100/1000 LAN", "Thunderbolt 1"
	Kind         string // normalized: "wifi", "ethernet", "bridge", "thunderbolt", "vpn", "virtual", "unknown"
}

// InterfaceInfoMap returns a map of device name -> InterfaceInfo for all
// known hardware ports on the system.
func InterfaceInfoMap() map[string]InterfaceInfo {
	switch runtime.GOOS {
	case "darwin":
		return ifaceInfoDarwin()
	default:
		return ifaceInfoLinux()
	}
}

// ifaceInfoDarwin parses `networksetup -listallhardwareports`.
//
// Output format:
//
//	Hardware Port: Wi-Fi
//	Device: en0
//	Ethernet Address: ...
func ifaceInfoDarwin() map[string]InterfaceInfo {
	out, err := exec.Command("networksetup", "-listallhardwareports").CombinedOutput()
	if err != nil {
		return nil
	}

	m := make(map[string]InterfaceInfo)
	var currentPort string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Hardware Port:") {
			currentPort = strings.TrimSpace(strings.TrimPrefix(line, "Hardware Port:"))
		}
		if strings.HasPrefix(line, "Device:") && currentPort != "" {
			dev := strings.TrimSpace(strings.TrimPrefix(line, "Device:"))
			m[dev] = InterfaceInfo{
				HardwarePort: currentPort,
				Kind:         classifyDarwinPort(currentPort),
			}
			currentPort = ""
		}
	}
	return m
}

func classifyDarwinPort(port string) string {
	p := strings.ToLower(port)
	switch {
	case strings.Contains(p, "wi-fi"):
		return "wifi"
	case strings.Contains(p, "thunderbolt bridge"):
		return "bridge"
	case strings.Contains(p, "thunderbolt"):
		return "thunderbolt"
	case strings.Contains(p, "ethernet") || strings.Contains(p, "lan"):
		return "ethernet"
	case strings.Contains(p, "vpn"):
		return "vpn"
	default:
		return "unknown"
	}
}

// ifaceInfoLinux inspects sysfs to determine interface types.
func ifaceInfoLinux() map[string]InterfaceInfo {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return nil
	}

	m := make(map[string]InterfaceInfo)
	for _, e := range entries {
		name := e.Name()
		info := InterfaceInfo{}

		// Check if wireless: /sys/class/net/<iface>/wireless exists
		if _, err := os.Stat("/sys/class/net/" + name + "/wireless"); err == nil {
			info.Kind = "wifi"
			info.HardwarePort = "Wi-Fi"
		} else if strings.HasPrefix(name, "wl") {
			// wlan0, wlp2s0, etc.
			info.Kind = "wifi"
			info.HardwarePort = "Wi-Fi"
		} else if strings.HasPrefix(name, "br") || strings.HasPrefix(name, "docker") || strings.HasPrefix(name, "virbr") {
			info.Kind = "bridge"
			info.HardwarePort = "Bridge"
		} else if strings.HasPrefix(name, "veth") {
			info.Kind = "virtual"
			info.HardwarePort = "Virtual Ethernet"
		} else if strings.HasPrefix(name, "tun") || strings.HasPrefix(name, "tap") || strings.HasPrefix(name, "wg") {
			info.Kind = "vpn"
			info.HardwarePort = "VPN/Tunnel"
		} else if strings.HasPrefix(name, "en") || strings.HasPrefix(name, "eth") {
			info.Kind = "ethernet"
			info.HardwarePort = "Ethernet"
		} else if name == "lo" {
			info.Kind = "loopback"
			info.HardwarePort = "Loopback"
		} else {
			info.Kind = "unknown"
			info.HardwarePort = name
		}

		m[name] = info
	}
	return m
}
