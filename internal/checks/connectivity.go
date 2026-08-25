package checks

import (
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charlesfeval/brown/internal/platform"
)

const (
	externalConnectivityTarget = "8.8.8.8"
	connectivityPingCount      = 300
	monitorPingCount           = 5
	monitorInterval            = 5 * time.Second
	jitterWarning              = 10 * time.Millisecond
)

var packetSummaryPattern = regexp.MustCompile(`(\d+)\s+packets transmitted,\s+(\d+)\s+(?:packets )?(?:received|recieved).*?([\d.]+)%\s+packet loss`)

type ConnectivityCheck struct{}

func (c *ConnectivityCheck) Name() string { return "Gateway vs Internet" }

func (c *ConnectivityCheck) Run() Result {
	gateway, err := platform.DefaultGateway()
	if err != nil || gateway == "" {
		message := "could not determine gateway"
		if err != nil {
			message += ": " + err.Error()
		}
		return Result{Name: c.Name(), Status: Fail, Message: message}
	}

	stats, err := compareConnectivity(gateway, connectivityPingCount)
	if err != nil {
		return Result{Name: c.Name(), Status: Fail, Message: err.Error()}
	}
	status, message := classifyConnectivity(stats)
	return Result{
		Name:    c.Name(),
		Status:  status,
		Message: message,
		Details: stats.details(),
	}
}

type pingStats struct {
	target      string
	transmitted int
	received    int
	loss        float64
	average     time.Duration
	stddev      time.Duration
	err         error
}

type connectivityStats struct {
	gateway  pingStats
	external pingStats
}

type adapterState struct {
	name string
	up   bool
}

func defaultAdapterState() adapterState {
	route, err := platform.DefaultRouteInterface()
	if err != nil || route == "" {
		return adapterState{}
	}
	iface, err := net.InterfaceByName(route)
	if err != nil {
		return adapterState{name: route}
	}
	return adapterState{name: route, up: iface.Flags&net.FlagUp != 0}
}

func compareConnectivity(gateway string, count int) (connectivityStats, error) {
	return compareConnectivityAt(gateway, count, 200)
}

func compareConnectivityAt(gateway string, count int, intervalMS int) (connectivityStats, error) {
	var result connectivityStats
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		result.gateway = pingAt(gateway, count, intervalMS)
	}()
	go func() {
		defer wg.Done()
		result.external = pingAt(externalConnectivityTarget, count, intervalMS)
	}()
	wg.Wait()

	if result.gateway.err != nil && result.external.err != nil {
		return result, fmt.Errorf("gateway and internet pings failed: %v; %v", result.gateway.err, result.external.err)
	}
	return result, nil
}

func ping(target string, count int) pingStats {
	return pingAt(target, count, 200)
}

func pingAt(target string, count int, intervalMS int) pingStats {
	stats := pingStats{target: target, transmitted: count}
	args := pingArgs(target, count, intervalMS)
	interval := time.Duration(intervalMS) * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(count)*interval+10*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "ping", args...).CombinedOutput()
	parsePingOutput(string(out), &stats)
	if ctx.Err() != nil {
		stats.err = ctx.Err()
	} else if err != nil && stats.received == 0 {
		stats.err = fmt.Errorf("%v", err)
	}
	return stats
}

func pingArgs(target string, count int, intervalMS int) []string {
	interval := fmt.Sprintf("%.3f", float64(intervalMS)/1000)
	if runtime.GOOS == "darwin" {
		return []string{"-c", strconv.Itoa(count), "-i", interval, "-W", "1000", target}
	}
	return []string{"-c", strconv.Itoa(count), "-i", interval, "-W", "1", target}
}

func parsePingOutput(output string, stats *pingStats) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "packet loss") {
			matches := packetSummaryPattern.FindStringSubmatch(line)
			if len(matches) == 4 {
				stats.transmitted, _ = strconv.Atoi(matches[1])
				stats.received, _ = strconv.Atoi(matches[2])
				stats.loss, _ = strconv.ParseFloat(matches[3], 64)
			}
		}
		if strings.Contains(line, "round-trip") || strings.HasPrefix(line, "rtt ") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			values := strings.Split(strings.TrimSpace(parts[1]), "/")
			if len(values) < 4 {
				continue
			}
			stats.average = parseMilliseconds(values[1])
			stats.stddev = parseMilliseconds(values[3])
		}
	}
}

func parseMilliseconds(value string) time.Duration {
	ms, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0
	}
	return time.Duration(ms * float64(time.Millisecond))
}

func (s connectivityStats) details() []string {
	return []string{
		formatPingStats(s.gateway),
		formatPingStats(s.external),
	}
}

func formatPingStats(stats pingStats) string {
	return fmt.Sprintf("%s: %.1f%% loss, avg %s, jitter/stddev %s",
		stats.target, stats.loss, stats.average.Round(time.Millisecond), stats.stddev.Round(time.Millisecond))
}

func classifyConnectivity(stats connectivityStats) (Status, string) {
	gatewayFailed := stats.gateway.loss > 0 || stats.gateway.err != nil
	externalFailed := stats.external.loss > 0 || stats.external.err != nil
	switch {
	case gatewayFailed && externalFailed:
		return Fail, "packet loss reaches the gateway: likely local network/Wi-Fi problem"
	case !gatewayFailed && externalFailed:
		return Fail, "gateway is reachable but 8.8.8.8 has packet loss: likely ISP/WAN problem"
	case gatewayFailed:
		return Fail, "gateway has packet loss: likely local network/Wi-Fi problem"
	case stats.gateway.stddev > jitterWarning || stats.external.stddev > jitterWarning:
		return Warn, "connectivity is available but latency jitter is high"
	default:
		return OK, "gateway and 8.8.8.8 are reachable"
	}
}

// Monitor runs concurrent five-ping connectivity checks until ctx is cancelled.
func Monitor(ctx context.Context, w io.Writer, exitOnError bool, reportPath string) error {
	gateway, err := platform.DefaultGateway()
	if err != nil || gateway == "" {
		if err == nil {
			err = fmt.Errorf("no default gateway found")
		}
		return err
	}

	ticker := time.NewTicker(monitorInterval)
	defer ticker.Stop()
	var samples []monitorSample
	for cycle := 1; ; cycle++ {
		timestamp := time.Now()
		adapter := defaultAdapterState()
		var stats connectivityStats
		var compareErr error
		var message string
		if !adapter.up {
			message = fmt.Sprintf("network adapter %s is disconnected", adapter.name)
			compareErr = fmt.Errorf("%s", message)
			samples = append(samples, monitorSample{at: timestamp, adapterConnected: false, failure: message})
		} else {
			stats, compareErr = compareConnectivity(gateway, monitorPingCount)
			message = "connectivity unavailable"
			if compareErr == nil {
				status, classifiedMessage := classifyConnectivity(stats)
				message = classifiedMessage
				sample := newMonitorSample(timestamp, stats)
				if status != OK {
					sample.failure = classifiedMessage
				}
				samples = append(samples, sample)
			} else {
				samples = append(samples, monitorSample{at: timestamp, adapterConnected: true, failure: compareErr.Error()})
			}
		}
		if reportPath != "" {
			if err := writeMonitorReport(reportPath, samples); err != nil {
				return fmt.Errorf("write report: %w", err)
			}
		}
		if compareErr != nil {
			fmt.Fprintf(w, "\n%s problem: %v\n", timestamp.Format("2006-01-02 15:04:05"), compareErr)
			if exitOnError {
				return compareErr
			}
		} else {
			status, message := classifyConnectivity(stats)
			if status == OK {
				fmt.Fprintf(w, "\r%s connection fine (gateway %s, 8.8.8.8 %s)   ", timestamp.Format("2006-01-02 15:04:05"), formatLatency(stats.gateway), formatLatency(stats.external))
			} else {
				fmt.Fprintf(w, "\n%s problem (cycle %d): %s — %s; %s\n", timestamp.Format("2006-01-02 15:04:05"), cycle, message, formatPingStats(stats.gateway), formatPingStats(stats.external))
				if exitOnError {
					return fmt.Errorf("%s", message)
				}
			}
		}
		select {
		case <-ctx.Done():
			fmt.Fprintln(w)
			return nil
		case <-ticker.C:
		}
	}
}

func formatLatency(stats pingStats) string {
	return fmt.Sprintf("%.0f%% loss, %s avg, %s jitter", stats.loss, stats.average.Round(time.Millisecond), stats.stddev.Round(time.Millisecond))
}
