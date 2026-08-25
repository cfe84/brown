package checks

import (
	"fmt"
	"net"
	"strings"

	"github.com/charlesfeval/brown/internal/platform"
)

// InterfaceCheck lists active non-loopback interfaces with their IPs
// and identifies which one carries the default route.
type InterfaceCheck struct{}

func (c *InterfaceCheck) Name() string { return "Network Interfaces" }

func (c *InterfaceCheck) Run() Result {
	ifaces, err := net.Interfaces()
	if err != nil {
		return Result{Name: c.Name(), Status: Fail, Message: "could not list interfaces: " + err.Error()}
	}

	defaultIface, _ := platform.DefaultRouteInterface()
	ifaceInfo := platform.InterfaceInfoMap()

	var details []string
	var count int
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil || len(addrs) == 0 {
			continue
		}
		count++

		label := iface.Name
		if info, ok := ifaceInfo[iface.Name]; ok && info.HardwarePort != "" {
			label = fmt.Sprintf("%s (%s)", iface.Name, info.HardwarePort)
		}

		ips := formatAddrs(addrs)
		line := fmt.Sprintf("%-34s %s", label, ips)
		if iface.Name == defaultIface {
			line += "  <- default route"
		}
		details = append(details, line)
	}

	if count == 0 {
		return Result{Name: c.Name(), Status: Fail, Message: "no active non-loopback interface found"}
	}

	msg := fmt.Sprintf("%d active", count)
	if defaultIface != "" {
		if info, ok := ifaceInfo[defaultIface]; ok && info.HardwarePort != "" {
			msg += fmt.Sprintf(", traffic via %s (%s)", defaultIface, info.HardwarePort)
		} else {
			msg += fmt.Sprintf(", traffic via %s", defaultIface)
		}
	}

	return Result{Name: c.Name(), Status: OK, Message: msg, Details: details}
}

// formatAddrs extracts the IP (without prefix length) from each address and
// joins them. It groups IPv4 first, then IPv6 link-locals are dimmed.
func formatAddrs(addrs []net.Addr) string {
	var v4, v6 []string
	for _, a := range addrs {
		ip, _, err := net.ParseCIDR(a.String())
		if err != nil {
			continue
		}
		if ip.To4() != nil {
			v4 = append(v4, ip.String())
		} else if ip.IsLinkLocalUnicast() {
			v6 = append(v6, ip.String()+" (link-local)")
		} else {
			v6 = append(v6, ip.String())
		}
	}
	all := append(v4, v6...)
	return strings.Join(all, ", ")
}
