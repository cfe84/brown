//go:build darwin || linux

package platform

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// DNSServer represents a configured nameserver and optionally which
// resolver scope it belongs to (macOS can have per-domain resolvers).
type DNSServer struct {
	IP    string
	Scope string // e.g. "default", "mdns", a domain name, or empty on Linux
}

// ConfiguredDNSServers returns the list of DNS servers the OS is configured to use.
func ConfiguredDNSServers() ([]DNSServer, error) {
	switch runtime.GOOS {
	case "darwin":
		return dnsDarwin()
	default:
		return dnsLinux()
	}
}

// dnsDarwin parses `scutil --dns` output.
// We look for resolver blocks and extract nameserver lines.
//
// Example block:
//
//	resolver #1
//	  domain   : example.com
//	  nameserver[0] : 192.168.1.1
//	  nameserver[1] : 8.8.8.8
func dnsDarwin() ([]DNSServer, error) {
	out, err := exec.Command("scutil", "--dns").CombinedOutput()
	if err != nil {
		return nil, err
	}

	var servers []DNSServer
	seen := make(map[string]bool)
	scope := ""

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "resolver ") {
			scope = "" // reset for each resolver block
		}
		if strings.HasPrefix(line, "domain") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				scope = strings.TrimSpace(parts[1])
			}
		}
		if strings.HasPrefix(line, "search domain") {
			// skip search domains — not nameservers
			continue
		}
		if strings.Contains(line, "nameserver") && strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				ip := strings.TrimSpace(parts[1])
				if ip == "" {
					continue
				}
				s := scope
				if s == "" {
					s = "default"
				}
				key := ip + "/" + s
				if seen[key] {
					continue
				}
				seen[key] = true
				servers = append(servers, DNSServer{IP: ip, Scope: s})
			}
		}
	}
	return servers, nil
}

// dnsLinux parses /etc/resolv.conf.
func dnsLinux() ([]DNSServer, error) {
	data, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return nil, err
	}

	var servers []DNSServer
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "nameserver") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				servers = append(servers, DNSServer{IP: fields[1]})
			}
		}
	}
	return servers, nil
}
