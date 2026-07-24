package cli

import (
	"fmt"
	"sort"
	"strings"
)

// serverAliases maps short names to full base URLs for the well-known servers.
// Plain HTTP (port 12347) avoids TLS overhead; HTTPS (port 12348) is encrypted.
var serverAliases = map[string]string{
	"gz-http":  "http://gz.10000gd.tech:12347",
	"gz-https": "https://gz.10000gd.tech:12348",
}

// defaultServer is the alias used when --server is not given. Plain HTTP is the
// default so hardware without AES acceleration is not crypto-bound; see the
// README's "Plain HTTP" section.
const defaultServer = "gz-http"

// resolveServer expands a known alias to its base URL, or returns the value
// unchanged when it is already a URL (anything containing "://").
func resolveServer(value string) string {
	if url, ok := serverAliases[value]; ok {
		return url
	}
	return value
}

// serverAliasHelp renders the alias list for the flag usage string, e.g.
// "gz-http=http://…, gz-https=https://…".
func serverAliasHelp() string {
	names := make([]string, 0, len(serverAliases))
	for name := range serverAliases {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, fmt.Sprintf("%s=%s", name, serverAliases[name]))
	}
	return strings.Join(parts, ", ")
}
