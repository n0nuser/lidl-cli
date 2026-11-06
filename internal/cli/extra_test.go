package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/n0nuser/lidl-cli/internal/config"
)

func TestWatchPrettyAndDelivery(t *testing.T) {
	var posts atomic.Int32
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer hook.Close()
	isolate(t, site(t, map[string]string{searchPath: "search_atornillador.json"}))
	run("config", "set", "sinks.hook.type", "webhook")
	run("config", "set", "sinks.hook.url", hook.URL)

	out, errOut, code := run("watch", "add", "search", "atornillador", "--name", "osb",
		"--notify", "hook", "--emit-initial", "--format", "pretty")
	if code != 0 {
		t.Fatalf("add: exit %d: %s", code, errOut)
	}
	if !strings.Contains(out, "WATCH") || !strings.Contains(out, "osb") {
		t.Fatalf("%s", out)
	}
	if posts.Load() != 48 {
		t.Fatalf("%d events delivered, want 48", posts.Load())
	}
	if pretty, _, code := run("watch", "events", "--format", "pretty"); code != 0 || !strings.Contains(pretty, "EVENT") {
		t.Fatalf("%s", pretty)
	}
}

func TestRootSetupPaths(t *testing.T) {
	isolate(t, site(t, map[string]string{searchPath: "search_atornillador.json"}))
	if _, _, code := run("help"); code != 0 {
		t.Fatalf("help: %d", code)
	}
	if _, _, code := run("nope"); code != 2 {
		t.Fatalf("unknown command: %d", code)
	}
	if _, _, code := run("config", "list", "--debug"); code != 0 {
		t.Fatalf("debug: %d", code)
	}
	// A corrupt config is a usage error.
	cfgDir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "lidl-cli")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte("watch = [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, code := run("search", "x"); code != 2 {
		t.Fatalf("corrupt config: %d", code)
	}
}

func TestConfigDocReadError(t *testing.T) {
	a := &App{Paths: config.Paths{ConfigFile: t.TempDir()}}
	if _, err := a.configDoc(); err == nil {
		t.Fatal("a directory read as a config")
	}
}

func TestConfirm(t *testing.T) {
	var errb bytes.Buffer
	a := &App{Interactive: false, Stderr: &errb}
	if err := a.confirm(false, "sure?"); err == nil {
		t.Fatal("no terminal and no --yes was accepted")
	}
	if err := a.confirm(true, "sure?"); err != nil {
		t.Fatalf("--yes: %v", err)
	}
	a = &App{Interactive: true, Stdin: strings.NewReader("n\n"), Stderr: &errb}
	if err := a.confirm(false, "sure?"); err == nil {
		t.Fatal("a no was accepted")
	}
	a = &App{Interactive: true, Stdin: strings.NewReader("y\n"), Stderr: &errb}
	if err := a.confirm(false, "sure?"); err != nil {
		t.Fatalf("yes: %v", err)
	}
}

func TestMoreCommandPaths(t *testing.T) {
	isolate(t, site(t, map[string]string{searchPath: "search_atornillador.json"}))
	// The pretty watch view, and the interval from config.
	if _, _, code := run("config", "set", "watch.interval", "1h"); code != 0 {
		t.Fatalf("set interval: %d", code)
	}
	pretty, errOut, code := run("watch", "add", "search", "osb", "--name", "cfg", "--format", "pretty")
	if code != 0 {
		t.Fatalf("add: %d %s", code, errOut)
	}
	if !strings.Contains(pretty, "cfg") || !strings.Contains(pretty, "TARGET") {
		t.Fatalf("%s", pretty)
	}
	if _, _, code := run("watch", "remove", "cfg"); code != 2 {
		t.Fatalf("remove without --yes: %d", code)
	}
	if _, _, code := run("watch", "run", "--interval", "1m"); code != 2 {
		t.Fatalf("short run interval: %d", code)
	}
	if _, _, code := run("search", "filters"); code != 2 {
		t.Fatalf("filters without a query: %d", code)
	}
}

func TestDeliveryFailureIsANotice(t *testing.T) {
	isolate(t, site(t, map[string]string{searchPath: "search_atornillador.json"}))
	run("config", "set", "sinks.hook.type", "webhook")
	run("config", "set", "sinks.hook.url", "http://127.0.0.1:1")
	_, errOut, code := run("watch", "add", "search", "osb", "--name", "w", "--notify", "hook", "--emit-initial")
	if code != 0 {
		t.Fatalf("add: %d %s", code, errOut)
	}
	if !strings.Contains(errOut, "sink hook") {
		t.Fatalf("no delivery notice: %q", errOut)
	}
}

// A watch with an unparseable configured interval is a usage error.
func TestWatchBadConfiguredInterval(t *testing.T) {
	isolate(t, "http://127.0.0.1:1")
	dir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "lidl-cli")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("[watch]\ninterval = \"abc\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, errOut, code := run("watch", "add", "search", "x", "--name", "w"); code != 2 ||
		!strings.Contains(errOut, "watch.interval") {
		t.Fatalf("bad configured interval: %d %q", code, errOut)
	}
}
