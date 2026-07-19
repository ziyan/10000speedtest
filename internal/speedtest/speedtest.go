// Package speedtest implements the throughput test used by 10000speedtest.
//
// The China Telecom Guangdong web client drives its test with many parallel
// HTTP connections:
//
//   - Download: GET  <server>/shmfile/<sizeMiB>?tag=<random>  -> server streams sizeMiB of octet-stream
//   - Upload:   POST <server>/upload?tag=<random>             -> client streams a body, server sinks it
//
// A Tester reproduces that behaviour: it opens Config.Connections parallel
// connections for Config.Duration and sums the bytes moved, reporting the
// throughput in Mbps.
package speedtest

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"io"
	mathrand "math/rand"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ziyan/10000speedtest/internal/deferutil"
	"github.com/ziyan/10000speedtest/internal/logging"
)

const (
	bytesPerMebibyte = 1024 * 1024
	// requestOrigin mirrors the header the browser sends; some backends check it for CORS.
	requestOrigin = "https://10000.gd.cn"
)

var log = logging.MustGetLogger("speedtest")

// Config describes a single speed-test run.
type Config struct {
	Server       string
	Connections  int
	Duration     time.Duration
	DownloadSize int
	UploadSize   int
	Insecure     bool
}

// Tester runs download and upload stages against a configured server.
type Tester struct {
	config Config
	client *http.Client
}

// New builds a Tester with an HTTP client tuned for the test server.
func New(config Config) *Tester {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.Insecure, //nolint:gosec // the test port serves a non-matching cert by design
			MinVersion:         tls.VersionTLS12,
			// The test server negotiates TLS 1.2 with the RSA cipher
			// AES256-GCM-SHA384, which Go does not offer by default. Enable the
			// RSA suites explicitly (alongside the modern ECDHE ones) so the
			// handshake succeeds.
			CipherSuites: []uint16{
				tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
		},
		MaxIdleConnsPerHost: config.Connections,
		ForceAttemptHTTP2:   false,
	}
	return &Tester{config: config, client: &http.Client{Transport: transport}}
}

// Download runs the download stage and returns the measured throughput in Mbps.
func (self *Tester) Download() float64 {
	return self.runStage("Download", self.downloadWorker)
}

// Upload runs the upload stage and returns the measured throughput in Mbps.
func (self *Tester) Upload() float64 {
	return self.runStage("Upload", self.uploadWorker)
}

// runStage launches Config.Connections worker goroutines that share an atomic
// byte counter, runs them for Config.Duration, prints a live readout, and
// returns the average throughput in Mbps.
func (self *Tester) runStage(name string, worker func(context.Context, *atomic.Int64)) float64 {
	ctx, cancel := context.WithTimeout(context.Background(), self.config.Duration)
	defer cancel()

	counter := &atomic.Int64{}
	start := time.Now()

	var waitGroup sync.WaitGroup
	for index := 0; index < self.config.Connections; index++ {
		waitGroup.Add(1)
		go func() {
			defer deferutil.Recover()
			defer waitGroup.Done()
			worker(ctx, counter)
		}()
	}

	reporterDone := make(chan struct{})
	go liveReporter(name, counter, start, reporterDone)

	waitGroup.Wait()
	close(reporterDone)

	elapsed := time.Since(start).Seconds()
	totalBytes := counter.Load()
	megabitsPerSecond := float64(totalBytes) * 8 / elapsed / 1e6
	fmt.Printf("\r%-10s %8.2f Mbps   (%s in %.1fs)%s\n",
		name+":", megabitsPerSecond, humanBytes(totalBytes), elapsed, "          ")
	return megabitsPerSecond
}

// liveReporter prints the instantaneous throughput roughly twice a second.
func liveReporter(name string, counter *atomic.Int64, start time.Time, done chan struct{}) {
	defer deferutil.Recover()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	previousBytes := int64(0)
	previousTime := start
	for {
		select {
		case <-done:
			return
		case now := <-ticker.C:
			currentBytes := counter.Load()
			interval := now.Sub(previousTime).Seconds()
			if interval <= 0 {
				continue
			}
			megabitsPerSecond := float64(currentBytes-previousBytes) * 8 / interval / 1e6
			fmt.Printf("\r%-10s %8.2f Mbps   ", name+":", megabitsPerSecond)
			previousBytes = currentBytes
			previousTime = now
		}
	}
}

// downloadWorker repeatedly downloads /shmfile/<N> until the context is
// cancelled, adding every received byte to the shared counter.
func (self *Tester) downloadWorker(ctx context.Context, counter *atomic.Int64) {
	writer := &countingWriter{counter: counter}
	buffer := make([]byte, 256*1024)
	for ctx.Err() == nil {
		url := fmt.Sprintf("%s/shmfile/%d?tag=%s", self.config.Server, self.config.DownloadSize, randomTag())
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return
		}
		response, err := self.client.Do(request)
		if err != nil {
			log.Debugf("download request failed: %v", err)
			continue
		}
		if _, err := io.CopyBuffer(writer, response.Body, buffer); err != nil {
			log.Debugf("download read failed: %v", err)
		}
		_ = response.Body.Close()
	}
}

// uploadWorker repeatedly posts a body to /upload until the context is
// cancelled, adding every sent byte to the shared counter.
func (self *Tester) uploadWorker(ctx context.Context, counter *atomic.Int64) {
	chunk := make([]byte, 128*1024)
	_, _ = rand.Read(chunk)
	totalBytes := int64(self.config.UploadSize) * bytesPerMebibyte

	for ctx.Err() == nil {
		body := &meteredReader{remaining: totalBytes, chunk: chunk, counter: counter}
		url := fmt.Sprintf("%s/upload?tag=%s", self.config.Server, randomTag())
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
		if err != nil {
			return
		}
		request.ContentLength = totalBytes
		request.Header.Set("Content-Type", "application/octet-stream")
		request.Header.Set("Origin", requestOrigin)

		response, err := self.client.Do(request)
		if err != nil {
			log.Debugf("upload request failed: %v", err)
			continue
		}
		if _, err := io.Copy(io.Discard, response.Body); err != nil {
			log.Debugf("upload response read failed: %v", err)
		}
		_ = response.Body.Close()
	}
}

// countingWriter is an io.Writer that discards its input while counting bytes.
type countingWriter struct {
	counter *atomic.Int64
}

func (self *countingWriter) Write(data []byte) (int, error) {
	self.counter.Add(int64(len(data)))
	return len(data), nil
}

// meteredReader streams exactly `remaining` bytes by repeating `chunk`, adding
// each byte handed to the transport to the shared counter as it goes.
type meteredReader struct {
	remaining int64
	chunk     []byte
	counter   *atomic.Int64
}

func (self *meteredReader) Read(buffer []byte) (int, error) {
	if self.remaining <= 0 {
		return 0, io.EOF
	}
	limit := len(buffer)
	if int64(limit) > self.remaining {
		limit = int(self.remaining)
	}
	written := 0
	for written < limit {
		written += copy(buffer[written:limit], self.chunk)
	}
	self.remaining -= int64(written)
	self.counter.Add(int64(written))
	return written, nil
}

// randomTag reproduces the ?tag= cache-buster the web client appends: a random
// float in [0,1) rendered like "0.5557143137032768".
func randomTag() string {
	return strconv.FormatFloat(mathrand.Float64(), 'f', -1, 64)
}

// humanBytes renders a byte count as a human-readable size.
func humanBytes(count int64) string {
	const unit = 1024
	if count < unit {
		return fmt.Sprintf("%d B", count)
	}
	value := float64(count)
	units := []string{"KiB", "MiB", "GiB", "TiB"}
	index := -1
	for value >= unit && index < len(units)-1 {
		value /= unit
		index++
	}
	return fmt.Sprintf("%.2f %s", value, units[index])
}
