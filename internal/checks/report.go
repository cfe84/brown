package checks

import (
	"fmt"
	"html"
	"os"
	"strings"
	"time"
)

type monitorSample struct {
	at               time.Time
	adapterConnected bool
	localSuccess     bool
	ispSuccess       bool
	localLatency     time.Duration
	ispLatency       time.Duration
	failure          string
}

type reportPoint struct {
	at           time.Time
	localPercent float64
	ispPercent   float64
	localLatency time.Duration
	ispLatency   time.Duration
}

func newMonitorSample(at time.Time, stats connectivityStats) monitorSample {
	return monitorSample{
		at:               at,
		adapterConnected: true,
		localSuccess:     stats.gateway.err == nil && stats.gateway.loss == 0,
		ispSuccess:       stats.external.err == nil && stats.external.loss == 0,
		localLatency:     stats.gateway.average,
		ispLatency:       stats.external.average,
	}
}

func writeMonitorReport(path string, samples []monitorSample) error {
	var localOK, ispOK int
	var localTimes, ispTimes []float64
	var failures []string
	var timeline, latency string
	points := aggregateSamples(samples, 25)
	chartWidth := maxInt(len(points)*30+60, 240)
	for _, sample := range samples {
		if sample.localSuccess {
			localOK++
		}
		if sample.ispSuccess {
			ispOK++
		}
		if sample.localLatency > 0 {
			localTimes = append(localTimes, float64(sample.localLatency.Microseconds())/1000)
		}
		if sample.ispLatency > 0 {
			ispTimes = append(ispTimes, float64(sample.ispLatency.Microseconds())/1000)
		}
		if sample.failure != "" {
			failures = append(failures, fmt.Sprintf("%s: %s", sample.at.Format("2006-01-02 15:04:05"), sample.failure))
		}
	}
	timeline = lineChart(points, false)
	latency = lineChart(points, true)
	total := len(samples)
	localPct, ispPct := percentage(localOK, total), percentage(ispOK, total)
	content := fmt.Sprintf(`<!doctype html>
<html><head><meta charset="utf-8"><title>Brown connectivity report</title>
<style>body{font:15px system-ui,sans-serif;max-width:1000px;margin:2rem auto;padding:0 1rem;color:#172033}svg{width:100%%;height:220px;border:1px solid #ddd;background:#fafafa}.local{color:#2563eb}.isp{color:#dc2626}table{border-collapse:collapse}td,th{padding:.4rem 1rem;border-bottom:1px solid #ddd;text-align:left}</style></head>
<body><h1>Brown connectivity report</h1><p>Samples: %d; last updated: %s</p>
<h2>Summary</h2><table><tr><th></th><th>Reliability</th><th>Average</th><th>Min</th><th>Max</th><th>Stddev</th></tr>
<tr><th class="local">Local gateway</th><td>%.1f%%</td>%s</tr><tr><th class="isp">ISP / 8.8.8.8</th><td>%.1f%%</td>%s</tr></table>
<h2>Timeline</h2><p><span class="local">● Local</span> <span class="isp">● ISP</span></p><h3>Reliability</h3><svg viewBox="0 0 %d 120">%s%s</svg>
<h3>Response time (ms)</h3><svg viewBox="0 0 %d 120">%s%s</svg>
<h2>Failures</h2>%s</body></html>`,
		total, time.Now().Format("2006-01-02 15:04:05"), localPct, metricCells(localTimes), ispPct, metricCells(ispTimes),
		chartWidth, reliabilityScale(), timeline, chartWidth, latencyScale(samples), latency, failureHTML(failures))
	return os.WriteFile(path, []byte(content), 0644)
}

func aggregateSamples(samples []monitorSample, limit int) []reportPoint {
	if len(samples) == 0 {
		return nil
	}
	buckets := len(samples)
	if buckets > limit {
		buckets = limit
	}
	points := make([]reportPoint, 0, buckets)
	for bucket := 0; bucket < buckets; bucket++ {
		start := bucket * len(samples) / buckets
		end := (bucket + 1) * len(samples) / buckets
		var point reportPoint
		var localOK, ispOK, localCount, ispCount, localLatencyCount, ispLatencyCount int
		for _, sample := range samples[start:end] {
			if point.at.IsZero() {
				point.at = sample.at
			}
			if sample.localSuccess {
				localOK++
			}
			if sample.ispSuccess {
				ispOK++
			}
			if sample.adapterConnected {
				localCount++
				ispCount++
			}
			if sample.localLatency > 0 {
				point.localLatency += sample.localLatency
				localLatencyCount++
			}
			if sample.ispLatency > 0 {
				point.ispLatency += sample.ispLatency
				ispLatencyCount++
			}
		}
		point.localPercent = percentage(localOK, localCount)
		point.ispPercent = percentage(ispOK, ispCount)
		if localLatencyCount > 0 {
			point.localLatency /= time.Duration(localLatencyCount)
		}
		if ispLatencyCount > 0 {
			point.ispLatency /= time.Duration(ispLatencyCount)
		}
		points = append(points, point)
	}
	return points
}

func lineChart(points []reportPoint, responseTime bool) string {
	var local, isp, hitAreas strings.Builder
	for i, point := range points {
		x := i*30 + 50
		localY, ispY := 0, 0
		if responseTime {
			localY = valueY(point.localLatency, points)
			ispY = valueY(point.ispLatency, points)
		} else {
			localY = percentageY(point.localPercent)
			ispY = percentageY(point.ispPercent)
		}
		if i > 0 {
			previous := points[i-1]
			previousLocalY, previousISPY := 0, 0
			if responseTime {
				previousLocalY = valueY(previous.localLatency, points)
				previousISPY = valueY(previous.ispLatency, points)
			} else {
				previousLocalY = percentageY(previous.localPercent)
				previousISPY = percentageY(previous.ispPercent)
			}
			local.WriteString(fmt.Sprintf(`<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#2563eb" stroke-width="2"/>`, x-30, previousLocalY, x, localY))
			isp.WriteString(fmt.Sprintf(`<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#dc2626" stroke-width="2"/>`, x-30, previousISPY, x, ispY))
		}
		hitAreas.WriteString(fmt.Sprintf(`<circle cx="%d" cy="%d" r="8" fill="transparent"><title>%s</title></circle>`, x, localY, pointHoverText(point, "Local gateway", point.localPercent, point.localLatency)))
		hitAreas.WriteString(fmt.Sprintf(`<circle cx="%d" cy="%d" r="8" fill="transparent"><title>%s</title></circle>`, x, ispY, pointHoverText(point, "ISP / 8.8.8.8", point.ispPercent, point.ispLatency)))
	}
	return axisMarkers(points) + local.String() + isp.String() + hitAreas.String()
}

func axisMarkers(points []reportPoint) string {
	if len(points) == 0 {
		return ""
	}
	var markers strings.Builder
	lastMarker := -1
	for i := 0; i < len(points); i += 5 {
		lastMarker = i
		markers.WriteString(axisMarker(points[i], i))
	}
	if lastMarker != len(points)-1 {
		markers.WriteString(axisMarker(points[len(points)-1], len(points)-1))
	}
	return markers.String()
}

func axisMarker(point reportPoint, index int) string {
	x := index*30 + 50
	return fmt.Sprintf(`<line x1="%d" y1="10" x2="%d" y2="95" stroke="#e5e7eb" stroke-dasharray="2,2"/><line x1="%d" y1="95" x2="%d" y2="98" stroke="#52606d"/><text x="%d" y="109" text-anchor="middle" fill="#52606d" font-size="8">%s</text>`,
		x, x, x, x, x, html.EscapeString(point.at.Format("01-02 15:04")))
}

func reliabilityScale() string {
	return `<g fill="#52606d" font-size="8"><text x="4" y="13">100%</text><text x="4" y="53">50%</text><text x="4" y="93">0%</text></g><g stroke="#d9e2ec" stroke-width="1"><line x1="40" y1="10" x2="100%" y2="10"/><line x1="40" y1="50" x2="100%" y2="50"/><line x1="40" y1="90" x2="100%" y2="90"/></g>`
}

func pointHoverText(point reportPoint, series string, reliability float64, latency time.Duration) string {
	response := "n/a"
	if latency > 0 {
		response = latency.Round(time.Millisecond).String()
	}
	return html.EscapeString(fmt.Sprintf("%s\n%s\nReliability: %.1f%%\nResponse: %s", point.at.Format("2006-01-02 15:04:05"), series, reliability, response))
}

func latencyScale(samples []monitorSample) string {
	max := time.Duration(1)
	for _, sample := range samples {
		if sample.localLatency > max {
			max = sample.localLatency
		}
		if sample.ispLatency > max {
			max = sample.ispLatency
		}
	}
	maxMS := float64(max.Microseconds()) / 1000
	return fmt.Sprintf(`<g fill="#52606d" font-size="8"><text x="4" y="93">0 ms</text><text x="4" y="53">%.1f ms</text><text x="4" y="13">%.1f ms</text></g><g stroke="#d9e2ec" stroke-width="1"><line x1="40" y1="90" x2="100%%" y2="90"/><line x1="40" y1="50" x2="100%%" y2="50"/><line x1="40" y1="10" x2="100%%" y2="10"/></g>`, maxMS/2, maxMS)
}

func percentage(ok, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(ok) * 100 / float64(total)
}

func metricCells(values []float64) string {
	if len(values) == 0 {
		return "<td colspan=\"4\">n/a</td>"
	}
	var sum, min, max float64
	min, max = values[0], values[0]
	for _, value := range values {
		sum += value
		if value < min {
			min = value
		}
		if value > max {
			max = value
		}
	}
	mean := sum / float64(len(values))
	var variance float64
	for _, value := range values {
		delta := value - mean
		variance += delta * delta
	}
	return fmt.Sprintf("<td>%.1f ms</td><td>%.1f ms</td><td>%.1f ms</td><td>%.1f ms</td>", mean, min, max, sqrt(variance/float64(len(values))))
}

func failureHTML(failures []string) string {
	if len(failures) == 0 {
		return "<p>No failures recorded.</p>"
	}
	var b strings.Builder
	b.WriteString("<ul>")
	for _, failure := range failures {
		fmt.Fprintf(&b, "<li>%s</li>", html.EscapeString(failure))
	}
	b.WriteString("</ul>")
	return b.String()
}

func reliabilityY(success bool) int {
	if success {
		return 20
	}
	return 80
}

func percentageY(value float64) int {
	return 90 - int(value/100*80)
}

func valueY(value time.Duration, samples []reportPoint) int {
	if value <= 0 {
		return 90
	}
	max := time.Duration(1)
	for _, sample := range samples {
		if sample.localLatency > max {
			max = sample.localLatency
		}
		if sample.ispLatency > max {
			max = sample.ispLatency
		}
	}
	return 90 - int(float64(value)/float64(max)*70)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func sqrt(value float64) float64 {
	if value <= 0 {
		return 0
	}
	x := value
	for i := 0; i < 10; i++ {
		x = (x + value/x) / 2
	}
	return x
}
