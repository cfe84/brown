package checks

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

const (
	bandwidthDuration = 10 * time.Second
	bandwidthWarmup   = 2 * time.Second
)

// BandwidthCheck measures streamed upload and download throughput against a
// Brown bandwidth receiver.
type BandwidthCheck struct {
	BaseURL string
}

func (c *BandwidthCheck) Name() string { return "Bandwidth" }

func (c *BandwidthCheck) Run() Result {
	upload, err := measureUpload(c.BaseURL)
	if err != nil {
		return Result{Name: c.Name(), Status: Fail, Message: "upload failed: " + err.Error()}
	}
	download, err := measureDownload(c.BaseURL)
	if err != nil {
		return Result{Name: c.Name(), Status: Fail, Message: "download failed: " + err.Error()}
	}
	return Result{
		Name:    c.Name(),
		Status:  OK,
		Message: "10-second bandwidth test complete",
		Details: []string{fmt.Sprintf("upload: %.1f Mbps average after %s warm-up", upload, bandwidthWarmup), fmt.Sprintf("download: %.1f Mbps average after %s warm-up", download, bandwidthWarmup)},
	}
}

func measureUpload(baseURL string) (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), bandwidthDuration+3*time.Second)
	defer cancel()
	reader, writer := io.Pipe()
	start := time.Now()
	var measured atomic.Int64
	go streamUpload(ctx, writer, start, &measured)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint(baseURL, "/upload"), reader)
	if err != nil {
		return 0, err
	}
	request.Header.Set("Content-Type", "application/octet-stream")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		return 0, fmt.Errorf("server returned %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	if err := drain(response.Body); err != nil {
		return 0, err
	}
	return throughput(measured.Load(), start.Add(bandwidthWarmup), time.Now()), nil
}

func streamUpload(ctx context.Context, writer *io.PipeWriter, start time.Time, measured *atomic.Int64) {
	defer writer.Close()
	buffer := make([]byte, 256*1024)
	stop := time.NewTimer(bandwidthDuration)
	defer stop.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-stop.C:
			return
		default:
		}
		n, err := writer.Write(buffer)
		if err != nil {
			return
		}
		if time.Since(start) > bandwidthWarmup {
			measured.Add(int64(n))
		}
	}
}

func measureDownload(baseURL string) (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), bandwidthDuration+3*time.Second)
	defer cancel()
	start := time.Now()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint(baseURL, "/download"), nil)
	if err != nil {
		return 0, err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("server returned %s", response.Status)
	}
	buffer := make([]byte, 256*1024)
	var measured int64
	for {
		n, readErr := response.Body.Read(buffer)
		now := time.Now()
		if now.Sub(start) > bandwidthWarmup {
			measured += int64(n)
		}
		if readErr != nil {
			if readErr == context.DeadlineExceeded || readErr == io.EOF {
				break
			}
			return 0, readErr
		}
	}
	return throughput(measured, start.Add(bandwidthWarmup), time.Now()), nil
}

func drain(reader io.Reader) error {
	_, err := io.Copy(io.Discard, reader)
	return err
}

func throughput(bytes int64, start, end time.Time) float64 {
	seconds := end.Sub(start).Seconds()
	if seconds <= 0 {
		return 0
	}
	return float64(bytes*8) / seconds / 1_000_000
}

func endpoint(baseURL, path string) string {
	return strings.TrimRight(baseURL, "/") + path
}
