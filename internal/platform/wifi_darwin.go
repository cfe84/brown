//go:build darwin

package platform

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type WiFiNeighbor struct {
	SSID, BSSID, Band, Channel, Width, RSSI, Security, PHY string
	RSSIValue, PrimaryChannel, WidthMHz                    int
}

func isWiFiField(key string) bool {
	switch key {
	case "PHY Mode", "Channel", "Country Code", "Network Type", "Security", "Signal / Noise", "Transmit Rate", "MCS Index", "Status", "BSSID":
		return true
	default:
		return false
	}
}

type WiFiInfo struct {
	SSID, Interface, BSSID, PHY, Band, Channel, Width, RSSI, Noise, SNR, TransmitRate, MCS, Security, Gateway string
	Enabled, Associated                                                                                       bool
	Neighbors                                                                                                 []WiFiNeighbor
	StrongNeighbors, SameChannelNeighbors, OverlappingNeighbors                                               int
	Findings                                                                                                  []string
}

func ActiveWiFi() (WiFiInfo, error) {
	route, err := defaultRouteDarwin()
	if err != nil || route.Interface == "" {
		return WiFiInfo{}, fmt.Errorf("could not determine active route interface")
	}
	ports, err := exec.Command("networksetup", "-listallhardwareports").Output()
	if err != nil {
		return WiFiInfo{}, fmt.Errorf("could not inspect Wi-Fi interface: %w", err)
	}
	wifiInterface := ""
	lines := strings.Split(string(ports), "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "Hardware Port: Wi-Fi" && i+1 < len(lines) {
			wifiInterface = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[i+1]), "Device:"))
		}
	}
	if wifiInterface == "" {
		wifiInterface = route.Interface
	}
	if route.Interface != wifiInterface {
		return WiFiInfo{}, fmt.Errorf("default route uses %s; active connection is not Wi-Fi", route.Interface)
	}
	power, powerErr := exec.Command("networksetup", "-getairportpower", wifiInterface).Output()
	info := WiFiInfo{Interface: wifiInterface, Gateway: route.Gateway, Enabled: powerErr == nil && strings.Contains(string(power), "On")}
	if !info.Enabled {
		return info, nil
	}
	profile, err := exec.Command("system_profiler", "SPAirPortDataType").Output()
	if err != nil {
		return WiFiInfo{}, fmt.Errorf("could not inspect Wi-Fi configuration: %w", err)
	}
	parseWiFiProfile(string(profile), &info)
	info.Associated = info.Associated || info.SSID != "" || info.BSSID != ""
	info.Findings = validateWiFi(info)
	return info, nil
}

func parseWiFiProfile(output string, info *WiFiInfo) {
	active := false
	var neighbor *WiFiNeighbor
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(raw)
		if line == "Status: Connected" {
			info.Associated = true
		}
		if strings.HasPrefix(line, "Current Network Information:") {
			active = true
			continue
		}
		if strings.HasPrefix(line, "Other Local Wi-Fi Networks:") {
			active = false
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if active {
			switch key {
			case "Network Name (SSID)":
				info.SSID = value
			case "Status":
				info.Associated = strings.EqualFold(value, "Connected")
			case "BSSID":
				info.BSSID = value
			case "PHY Mode":
				info.PHY = value
			case "Channel":
				info.Channel, info.Band, info.Width = parseChannel(value)
			case "Signal / Noise":
				parts := strings.Split(value, " / ")
				if len(parts) == 2 {
					info.RSSI, info.Noise = parts[0], parts[1]
					info.SNR = fmt.Sprintf("%d dB", parseDBM(parts[0])-parseDBM(parts[1]))
				}
			case "Transmit Rate":
				info.TransmitRate = value
			case "MCS Index":
				info.MCS = value
			case "Security":
				info.Security = value
			}
			// Privacy-redacted profiler output replaces the SSID key with a
			// placeholder, so the first entry under the active network is
			// still sufficient to establish association.
			if info.SSID == "" && !isWiFiField(key) {
				info.SSID = key
			}
		} else {
			if key != "" && !strings.Contains(strings.ToLower(key), "network") {
				if neighbor == nil {
					neighbor = &WiFiNeighbor{SSID: key}
					info.Neighbors = append(info.Neighbors, *neighbor)
				}
				updateNeighbor(&info.Neighbors[len(info.Neighbors)-1], key, value)
			}
		}
	}
	countRFConflicts(info)
}

func updateNeighbor(n *WiFiNeighbor, key, value string) {
	switch key {
	case "BSSID":
		n.BSSID = value
	case "Channel":
		n.Channel, n.Band, n.Width = parseChannel(value)
		n.PrimaryChannel = parseDBM(n.Channel)
	case "RSSI":
		n.RSSI = value
		n.RSSIValue = parseDBM(value)
	case "Security":
		n.Security = value
	case "PHY Mode":
		n.PHY = value
	}
}

func parseChannel(value string) (string, string, string) {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return value, "", ""
	}
	channel := fields[0]
	band := ""
	if strings.Contains(value, "2.4GHz") {
		band = "2.4GHz"
	} else if strings.Contains(value, "5GHz") {
		band = "5GHz"
	} else if strings.Contains(value, "6GHz") {
		band = "6GHz"
	}
	width := ""
	for _, field := range fields {
		if strings.HasSuffix(field, "MHz") {
			width = field
			break
		}
	}
	return channel, band, width
}

func parseDBM(value string) int {
	re := regexp.MustCompile(`-?\d+`)
	match := re.FindString(value)
	n, _ := strconv.Atoi(match)
	return n
}

func countRFConflicts(info *WiFiInfo) {
	active := parseDBM(info.Channel)
	for _, n := range info.Neighbors {
		if n.RSSIValue >= -70 {
			info.StrongNeighbors++
		}
		if n.PrimaryChannel == active && active != 0 {
			info.SameChannelNeighbors++
		}
		if n.PrimaryChannel == active && n.RSSIValue >= -70 {
			info.OverlappingNeighbors++
		}
	}
}

func validateWiFi(info WiFiInfo) []string {
	var findings []string
	if strings.HasPrefix(info.Band, "2.4") {
		if info.Channel != "1" && info.Channel != "6" && info.Channel != "11" {
			findings = append(findings, "warning: 2.4GHz is not on channel 1, 6, or 11; prefer 20MHz on one of those channels")
		}
		if strings.Contains(info.Width, "40") {
			findings = append(findings, "warning: 2.4GHz uses 40MHz; prefer 20MHz in residential RF environments")
		}
	} else if strings.HasPrefix(info.Band, "5") && parseDBM(info.RSSI) < -70 {
		findings = append(findings, "warning: 5GHz RSSI is weaker than -70 dBm; try repositioning the access point or client")
	}
	if info.OverlappingNeighbors > 0 {
		findings = append(findings, "warning: strong neighboring networks overlap the active channel")
	}
	if strings.Contains(strings.ToLower(info.Security), "open") || strings.Contains(strings.ToLower(info.Security), "wep") || strings.Contains(strings.ToLower(info.Security), "tkip") {
		findings = append(findings, "critical: replace open/WEP/TKIP security with WPA2-AES or WPA3-Personal")
	}
	return findings
}
