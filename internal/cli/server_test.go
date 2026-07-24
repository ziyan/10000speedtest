package cli

import (
	"strings"
	"testing"
)

// TestResolveServer checks alias expansion and URL pass-through.
func TestResolveServer(t *testing.T) {
	cases := map[string]string{
		"gz-http":                       "http://gz.10000gd.tech:12347",
		"gz-https":                      "https://gz.10000gd.tech:12348",
		"http://example.com:9":          "http://example.com:9", // already a URL
		"https://gz.10000gd.tech:12348": "https://gz.10000gd.tech:12348",
		"unknown-alias":                 "unknown-alias", // pass through unchanged
	}
	for input, want := range cases {
		if got := resolveServer(input); got != want {
			t.Errorf("resolveServer(%q) = %q, want %q", input, got, want)
		}
	}
}

// TestDefaultServerIsPlainHTTP guards the deliberate default: plain HTTP, so
// AES-less hardware is not crypto-bound.
func TestDefaultServerIsPlainHTTP(t *testing.T) {
	resolved := resolveServer(defaultServer)
	if !strings.HasPrefix(resolved, "http://") {
		t.Fatalf("default server should be plain HTTP, got %q", resolved)
	}
}
