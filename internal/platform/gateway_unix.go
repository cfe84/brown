//go:build darwin || linux

package platform

import (
	"os/exec"
	"runtime"
	"strings"
)

// DefaultRoute holds the gateway IP and the interface it routes through.
type DefaultRoute struct {
	Gateway   string
	Interface string
}

// DefaultGateway returns the default gateway IP by shelling out to platform
// route commands.
func DefaultGateway() (string, error) {
	r, err := defaultRoute()
	if err != nil {
		return "", err
	}
	return r.Gateway, nil
}

// DefaultRouteInterface returns the name of the interface carrying the default route.
func DefaultRouteInterface() (string, error) {
	r, err := defaultRoute()
	if err != nil {
		return "", err
	}
	return r.Interface, nil
}

func defaultRoute() (DefaultRoute, error) {
	switch runtime.GOOS {
	case "darwin":
		return defaultRouteDarwin()
	default:
		return defaultRouteLinux()
	}
}

func defaultRouteDarwin() (DefaultRoute, error) {
	// `route -n get default` prints a block with "gateway:" and "interface:" lines.
	out, err := exec.Command("route", "-n", "get", "default").CombinedOutput()
	if err != nil {
		return DefaultRoute{}, err
	}
	var r DefaultRoute
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "gateway:") {
			r.Gateway = strings.TrimSpace(strings.TrimPrefix(line, "gateway:"))
		}
		if strings.HasPrefix(line, "interface:") {
			r.Interface = strings.TrimSpace(strings.TrimPrefix(line, "interface:"))
		}
	}
	return r, nil
}

func defaultRouteLinux() (DefaultRoute, error) {
	// `ip route` first line is typically "default via <gw> dev <iface> ..."
	out, err := exec.Command("ip", "route", "show", "default").CombinedOutput()
	if err != nil {
		return DefaultRoute{}, err
	}
	var r DefaultRoute
	fields := strings.Fields(string(out))
	for i, f := range fields {
		if f == "via" && i+1 < len(fields) {
			r.Gateway = fields[i+1]
		}
		if f == "dev" && i+1 < len(fields) {
			r.Interface = fields[i+1]
		}
	}
	return r, nil
}
