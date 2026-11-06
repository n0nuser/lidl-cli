package cli

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixture reads a captured response from the api package's testdata.
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "api", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func gzipped(b []byte) []byte {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	w.Write(b)
	w.Close()
	return buf.Bytes()
}

// site serves fixtures by path ("404" answers a not-found page). A handler
// func lets a test see the request too.
func site(t *testing.T, routes map[string]string) string {
	t.Helper()
	return siteFunc(t, func(w http.ResponseWriter, r *http.Request) {
		name, ok := routes[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if name == "404" {
			http.NotFound(w, r)
			return
		}
		body := fixture(t, name)
		if strings.HasSuffix(r.URL.Path, ".gz") {
			body = gzipped(body)
		}
		w.Write(body)
	})
}

func siteFunc(t *testing.T, h http.HandlerFunc) string {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv.URL
}

// isolate points every path and the site at throwaway locations.
func isolate(t *testing.T, base string) {
	t.Helper()
	for _, k := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME"} {
		t.Setenv(k, t.TempDir())
	}
	t.Setenv("LIDL_BASE_URL", base)
	t.Setenv("LIDL_CONFIG", "")
}

func run(args ...string) (string, string, int) { return runWith("", args...) }

// runWith feeds stdin to one invocation.
func runWith(stdin string, args ...string) (string, string, int) {
	var out, errb bytes.Buffer
	code := Execute("test", args, strings.NewReader(stdin), &out, &errb)
	return out.String(), errb.String(), code
}

// decode reads a JSON document the CLI printed.
func decode[T any](t *testing.T, out string) T {
	t.Helper()
	var v T
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	return v
}
