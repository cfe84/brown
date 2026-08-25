package checks

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/charlesfeval/brown/internal/platform"
)

// DNSCheck verifies that DNS resolution is working and shows configured nameservers.
type DNSCheck struct{}

func (c *DNSCheck) Name() string { return "DNS Resolution" }

func (c *DNSCheck) Run() Result {
	host := "dns.google" // a reliable, well-known hostname

	start := time.Now()
	addrs, err := net.LookupHost(host)
	elapsed := time.Since(start)

	if err != nil {
		return Result{Name: c.Name(), Status: Fail, Message: fmt.Sprintf("lookup %s failed: %v", host, err)}
	}
	if len(addrs) == 0 {
		return Result{Name: c.Name(), Status: Fail, Message: fmt.Sprintf("lookup %s returned no addresses", host)}
	}

	msg := fmt.Sprintf("%s -> %s (%s)", host, addrs[0], elapsed.Round(time.Millisecond))

	details := dnsServerDetails()

	return Result{Name: c.Name(), Status: OK, Message: msg, Details: details}
}

func dnsServerDetails() []string {
	servers, err := platform.ConfiguredDNSServers()
	if err != nil || len(servers) == 0 {
		return []string{"could not determine configured DNS servers"}
	}

	// Separate default (primary) servers from scoped (split-DNS) ones.
	var primaryIPs []string
	scopedCount := 0
	seenPrimary := make(map[string]bool)
	scopedIPs := make(map[string]bool)

	for _, s := range servers {
		scope := s.Scope
		if scope == "" || scope == "default" {
			if !seenPrimary[s.IP] {
				seenPrimary[s.IP] = true
				primaryIPs = append(primaryIPs, s.IP)
			}
		} else {
			if !scopedIPs[s.IP] {
				scopedIPs[s.IP] = true
			}
			scopedCount++
		}
	}

	var details []string
	if len(primaryIPs) > 0 {
		details = append(details, "servers: "+strings.Join(primaryIPs, ", "))
	}
	if scopedCount > 0 {
		var uniqueScoped []string
		for ip := range scopedIPs {
			uniqueScoped = append(uniqueScoped, ip)
		}
		details = append(details, fmt.Sprintf(
			"+ %d scoped resolver(s) via %s",
			scopedCount, strings.Join(uniqueScoped, ", "),
		))
	}
	return details
}
