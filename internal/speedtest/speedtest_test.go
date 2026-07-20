package speedtest

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ziyan/10000speedtest/internal/logging"
)

// TestMain quiets the package logger so worker debug messages do not clutter
// test output.
func TestMain(runner *testing.M) {
	logging.Setup("error")
	os.Exit(runner.Run())
}

func testConfig(server string) Config {
	return Config{
		Server:       server,
		Connections:  2,
		Duration:     200 * time.Millisecond,
		DownloadSize: 1,
		UploadSize:   1,
		Insecure:     true,
	}
}

func mustNew(t *testing.T, config Config) *Tester {
	t.Helper()
	tester, err := New(config)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return tester
}

// TestResolveLocalAddress covers source-IP parsing, loopback lookup by name,
// and the not-found error.
func TestResolveLocalAddress(t *testing.T) {
	address, err := resolveLocalAddress("127.0.0.1")
	if err != nil || address.IP.String() != "127.0.0.1" {
		t.Fatalf("expected 127.0.0.1, got %v (err %v)", address, err)
	}
	if _, err := resolveLocalAddress("definitely-not-an-interface"); err == nil {
		t.Fatal("expected an error for an unknown interface")
	}
}

// runWorkerUntilBytes runs worker until it has counted at least one byte, then
// cancels it and returns the total. It stops as soon as data flows (rather than
// waiting out a fixed deadline), so the assertion does not depend on timing; the
// 5s ceiling only trips if the worker never transfers anything.
func runWorkerUntilBytes(t *testing.T, worker func(context.Context, *atomic.Int64)) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	counter := &atomic.Int64{}
	done := make(chan struct{})
	go func() {
		worker(ctx, counter)
		close(done)
	}()

	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for counter.Load() == 0 {
		select {
		case <-ctx.Done():
			<-done
			return counter.Load()
		case <-ticker.C:
		}
	}
	cancel()
	<-done
	return counter.Load()
}

// TestDownloadWorkerCountsBytes verifies that a successful download is counted.
func TestDownloadWorkerCountsBytes(t *testing.T) {
	payload := make([]byte, 64*1024)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !strings.HasPrefix(request.URL.Path, "/shmfile/") {
			http.NotFound(writer, request)
			return
		}
		for index := 0; index < 16; index++ {
			if _, err := writer.Write(payload); err != nil {
				return
			}
		}
	}))
	defer server.Close()

	tester := mustNew(t, testConfig(server.URL))
	if runWorkerUntilBytes(t, tester.downloadWorker) == 0 {
		t.Fatal("expected downloaded bytes to be counted, got 0")
	}
}

// TestDownloadWorkerSkipsNon200 verifies that an error response is not counted
// as download throughput.
func TestDownloadWorkerSkipsNon200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Error(writer, "this error body must not be counted", http.StatusInternalServerError)
	}))
	defer server.Close()

	tester := mustNew(t, testConfig(server.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	counter := &atomic.Int64{}
	tester.downloadWorker(ctx, counter)

	if counter.Load() != 0 {
		t.Fatalf("expected 0 bytes counted for non-200 responses, got %d", counter.Load())
	}
}

// TestUploadWorkerCountsBytes verifies that a successful upload is counted.
func TestUploadWorkerCountsBytes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tester := mustNew(t, testConfig(server.URL))
	if runWorkerUntilBytes(t, tester.uploadWorker) == 0 {
		t.Fatal("expected uploaded bytes to be counted, got 0")
	}
}

// TestMeteredReaderProducesExactSize verifies the reader emits exactly the
// requested number of bytes and counts every one of them.
func TestMeteredReaderProducesExactSize(t *testing.T) {
	const total = 3*128*1024 + 7
	counter := &atomic.Int64{}
	reader := &meteredReader{remaining: total, chunk: make([]byte, 128*1024), counter: counter}

	buffer := make([]byte, 4096)
	read := int64(0)
	for {
		count, err := reader.Read(buffer)
		read += int64(count)
		if err != nil {
			break
		}
	}

	if read != total {
		t.Fatalf("expected %d bytes read, got %d", total, read)
	}
	if counter.Load() != total {
		t.Fatalf("expected counter to reach %d, got %d", total, counter.Load())
	}
}
