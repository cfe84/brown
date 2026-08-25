package checks

import (
	"fmt"
	"strings"

	"github.com/charlesfeval/brown/internal/platform"
)

// WiFiCheck validates the active Wi-Fi link and performs a local stability test.
type WiFiCheck struct{}

func (c *WiFiCheck) Name() string { return "Wi-Fi Validation" }

func (c *WiFiCheck) Run() Result {
	info, err := platform.ActiveWiFi()
	if err != nil {
		return Result{Name: c.Name(), Status: Skip, Message: err.Error()}
	}
	if !info.Enabled {
		return Result{Name: c.Name(), Status: Fail, Message: fmt.Sprintf("Wi-Fi adapter %s is disabled", info.Interface)}
	}
	if !info.Associated {
		return Result{Name: c.Name(), Status: Fail, Message: fmt.Sprintf("Wi-Fi adapter %s is not associated with an access point", info.Interface)}
	}

	stats, stabilityErr := compareConnectivityAt(info.Gateway, 200, 500)
	status, message := wifiVerdict(info, stats, stabilityErr)
	details := []string{
		fmt.Sprintf("link: %s, %s, channel %s, width %s, RSSI %s, noise %s, SNR %s, TX rate %s",
			valueOrNA(info.PHY), valueOrNA(info.Band), valueOrNA(info.Channel), valueOrNA(info.Width),
			valueOrNA(info.RSSI), valueOrNA(info.Noise), valueOrNA(info.SNR), valueOrNA(info.TransmitRate)),
		fmt.Sprintf("security: %s, BSSID: %s, gateway: %s via %s", valueOrNA(info.Security), valueOrNA(info.BSSID), info.Gateway, info.Interface),
		fmt.Sprintf("RF environment: %d visible networks, %d strong, %d same-channel, %d overlapping active block",
			len(info.Neighbors), info.StrongNeighbors, info.SameChannelNeighbors, info.OverlappingNeighbors),
	}
	details = append(details, info.Findings...)
	if stabilityErr == nil {
		details = append(details, fmt.Sprintf("stability: %s; %s", formatPingStats(stats.gateway), formatPingStats(stats.external)))
	} else {
		details = append(details, "stability: "+stabilityErr.Error())
	}
	return Result{Name: c.Name(), Status: status, Message: message, Details: details}
}

func wifiVerdict(info platform.WiFiInfo, stats connectivityStats, stabilityErr error) (Status, string) {
	if stabilityErr != nil {
		return Fail, "Wi-Fi stability test could not complete"
	}
	if stats.gateway.loss > 3 {
		return Fail, "local Wi-Fi link has severe packet loss"
	}
	if stats.gateway.loss > 0 {
		return Warn, "local Wi-Fi link has packet loss"
	}
	if stats.external.loss > 3 {
		return Warn, "Wi-Fi is stable locally but the external path has severe loss"
	}
	if len(info.Findings) > 0 {
		return Warn, "Wi-Fi configuration has findings requiring attention"
	}
	return OK, "Wi-Fi configuration and local stability look healthy"
}

func valueOrNA(value string) string {
	if strings.TrimSpace(value) == "" {
		return "n/a"
	}
	return value
}
