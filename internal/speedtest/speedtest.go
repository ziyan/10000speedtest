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
	"net"
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
	Chart        bool
	JSONOutput   bool
	// Interfaces binds outgoing connections to local network interfaces (by name)
	// or source IPs. Each interface runs Connections connections and the results
	// are aggregated. Empty uses the default route.
	Interfaces []string
}

// InterfaceResult holds one interface's contribution to a stage.
type InterfaceResult struct {
	Interface         string
	Bytes             int64
	MegabitsPerSecond float64
}

// Result holds the measured throughput of one stage. When more than one
// interface is tested, PerInterface breaks the total down per interface.
type Result struct {
	Name              string
	Bytes             int64
	Elapsed           time.Duration
	MegabitsPerSecond float64
	PerInterface      []InterfaceResult
}

// boundClient is an HTTP client bound to a particular interface (or the default
// route when name is empty).
type boundClient struct {
	name   string
	client *http.Client
}

// Tester runs download and upload stages against a configured server, over one
// HTTP client per configured interface.
type Tester struct {
	config  Config
	clients []boundClient
}

// New builds a Tester with one HTTP client per configured interface. It returns
// an error if any interface cannot be resolved to a local address.
func New(config Config) (*Tester, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: config.Insecure, //nolint:gosec // the test port serves a non-matching cert by design
		MinVersion:         tls.VersionTLS12,
		// The test server negotiates TLS 1.2 with the RSA cipher
		// AES256-GCM-SHA384, which Go does not offer by default. Enable the RSA
		// suites explicitly (alongside the modern ECDHE ones) so the handshake
		// succeeds.
		CipherSuites: []uint16{
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}

	build := func(networkInterface string) (boundClient, error) {
		dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
		if networkInterface != "" {
			localAddress, err := resolveLocalAddress(networkInterface)
			if err != nil {
				return boundClient{}, err
			}
			dialer.LocalAddr = localAddress
			log.Debugf("binding connections to %s (%s)", networkInterface, localAddress)
		}
		transport := &http.Transport{
			DialContext:         dialer.DialContext,
			TLSClientConfig:     tlsConfig,
			MaxIdleConnsPerHost: config.Connections,
			ForceAttemptHTTP2:   false,
		}
		return boundClient{name: networkInterface, client: &http.Client{Transport: transport}}, nil
	}

	var clients []boundClient
	if len(config.Interfaces) == 0 {
		client, _ := build("") // building the default client cannot fail
		clients = []boundClient{client}
	} else {
		for _, networkInterface := range config.Interfaces {
			client, err := build(networkInterface)
			if err != nil {
				return nil, err
			}
			clients = append(clients, client)
		}
	}
	return &Tester{config: config, clients: clients}, nil
}

// resolveLocalAddress turns an interface name or source IP into a local TCP
// address to bind outgoing connections to. Binding by interface name uses the
// interface's first IPv4 address; source-based routing must then send that
// address out the intended NIC.
func resolveLocalAddress(value string) (*net.TCPAddr, error) {
	if parsed := net.ParseIP(value); parsed != nil {
		return &net.TCPAddr{IP: parsed}, nil
	}
	networkInterface, err := net.InterfaceByName(value)
	if err != nil {
		return nil, fmt.Errorf("speedtest: interface %q not found: %w", value, err)
	}
	addresses, err := networkInterface.Addrs()
	if err != nil {
		return nil, fmt.Errorf("speedtest: cannot read addresses of interface %q: %w", value, err)
	}
	for _, address := range addresses {
		ipNet, ok := address.(*net.IPNet)
		if ok && ipNet.IP.To4() != nil && !ipNet.IP.IsLoopback() {
			return &net.TCPAddr{IP: ipNet.IP}, nil
		}
	}
	return nil, fmt.Errorf("speedtest: interface %q has no usable IPv4 address", value)
}

// Download runs the download stage and returns its measured throughput.
func (self *Tester) Download() Result {
	return self.runStage("Download", self.downloadWorker)
}

// Upload runs the upload stage and returns its measured throughput.
func (self *Tester) Upload() Result {
	return self.runStage("Upload", self.uploadWorker)
}

// runStage launches Config.Connections worker goroutines per interface, each
// counting into its own atomic counter, runs them for Config.Duration, shows
// live combined progress, and returns the aggregated Result.
func (self *Tester) runStage(name string, worker func(context.Context, *http.Client, *atomic.Int64)) Result {
	ctx, cancel := context.WithTimeout(context.Background(), self.config.Duration)
	defer cancel()

	counters := make([]*atomic.Int64, len(self.clients))
	for index := range counters {
		counters[index] = &atomic.Int64{}
	}
	start := time.Now()

	var waitGroup sync.WaitGroup
	for index, bound := range self.clients {
		client := bound.client
		counter := counters[index]
		for connection := 0; connection < self.config.Connections; connection++ {
			waitGroup.Add(1)
			go func() {
				defer deferutil.Recover()
				defer waitGroup.Done()
				worker(ctx, client, counter)
			}()
		}
	}

	render := newRenderer(name, self.config)
	reporterDone := make(chan struct{})
	reporterExited := make(chan struct{})
	go liveReporter(counters, start, reporterDone, reporterExited, render)

	waitGroup.Wait()
	// Stop the reporter and wait for it to finish drawing so its last frame
	// cannot land after the summary line and corrupt the output.
	close(reporterDone)
	<-reporterExited

	elapsed := time.Since(start)
	return self.buildResult(name, elapsed, counters, render)
}

// buildResult aggregates per-interface counters into a Result and finalizes the
// renderer.
func (self *Tester) buildResult(name string, elapsed time.Duration, counters []*atomic.Int64, render renderer) Result {
	seconds := elapsed.Seconds()
	total := int64(0)
	perInterface := make([]InterfaceResult, 0, len(self.clients))
	for index, bound := range self.clients {
		bytes := counters[index].Load()
		total += bytes
		if bound.name != "" {
			perInterface = append(perInterface, InterfaceResult{
				Interface:         bound.name,
				Bytes:             bytes,
				MegabitsPerSecond: float64(bytes) * 8 / seconds / 1e6,
			})
		}
	}
	result := Result{
		Name:              name,
		Bytes:             total,
		Elapsed:           elapsed,
		MegabitsPerSecond: float64(total) * 8 / seconds / 1e6,
		PerInterface:      perInterface,
	}
	render.finish(result)
	return result
}

// sumCounters returns the total bytes counted across every interface.
func sumCounters(counters []*atomic.Int64) int64 {
	total := int64(0)
	for _, counter := range counters {
		total += counter.Load()
	}
	return total
}

// liveReporter samples the combined instantaneous throughput a few times a
// second and feeds each sample to the renderer. It closes exited when it
// returns so runStage can wait for it to stop drawing.
func liveReporter(counters []*atomic.Int64, start time.Time, done, exited chan struct{}, render renderer) {
	defer deferutil.Recover()
	defer close(exited)
	ticker := time.NewTicker(reportInterval)
	defer ticker.Stop()

	previousBytes := int64(0)
	previousTime := start
	for {
		select {
		case <-done:
			return
		case now := <-ticker.C:
			currentBytes := sumCounters(counters)
			interval := now.Sub(previousTime).Seconds()
			if interval <= 0 {
				continue
			}
			megabitsPerSecond := float64(currentBytes-previousBytes) * 8 / interval / 1e6
			render.sample(megabitsPerSecond)
			previousBytes = currentBytes
			previousTime = now
		}
	}
}

// downloadWorker repeatedly downloads /shmfile/<N> until the context is
// cancelled, adding every received byte to the shared counter.
func (self *Tester) downloadWorker(ctx context.Context, client *http.Client, counter *atomic.Int64) {
	writer := &countingWriter{counter: counter}
	buffer := make([]byte, 256*1024)
	for ctx.Err() == nil {
		url := fmt.Sprintf("%s/shmfile/%d?tag=%s", self.config.Server, self.config.DownloadSize, randomTag())
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return
		}
		response, err := client.Do(request)
		if err != nil {
			log.Debugf("download request failed: %v", err)
			continue
		}
		if response.StatusCode != http.StatusOK {
			// An error page is not payload; skip it so its body is not counted
			// as download throughput.
			log.Debugf("download returned unexpected status %s", response.Status)
			_ = response.Body.Close()
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
func (self *Tester) uploadWorker(ctx context.Context, client *http.Client, counter *atomic.Int64) {
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

		response, err := client.Do(request)
		if err != nil {
			log.Debugf("upload request failed: %v", err)
			continue
		}
		if response.StatusCode != http.StatusOK {
			log.Debugf("upload returned unexpected status %s", response.Status)
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
//
// Bytes are counted when the transport reads them, which matches how the
// browser test and typical speed tests measure upload. A small tail that is
// buffered by the transport or the kernel but not yet on the wire when the
// deadline fires is therefore included; over a multi-second run across many
// connections this is negligible.
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
