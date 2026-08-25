//go:build !darwin

package platform

import "fmt"

type WiFiNeighbor struct{}

type WiFiInfo struct {
	Interface, Gateway                                          string
	Enabled, Associated                                         bool
	Findings                                                    []string
	Neighbors                                                   []WiFiNeighbor
	StrongNeighbors, SameChannelNeighbors, OverlappingNeighbors int
}

func ActiveWiFi() (WiFiInfo, error) {
	return WiFiInfo{}, fmt.Errorf("Wi-Fi validation is currently supported on macOS only")
}
