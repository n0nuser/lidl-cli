package cli

import (
	"strings"
	"testing"

	"github.com/n0nuser/lidl-cli/internal/config"
)

func TestConfigCommands(t *testing.T) {
	isolate(t, "http://127.0.0.1:1")
	out, errOut, code := run("config", "path")
	if code != 0 || !strings.Contains(out, "config.toml") || !strings.Contains(out, "state.db") {
		t.Fatalf("path: %d %s %s", code, out, errOut)
	}
	if out, _, code := run("config", "list"); code != 0 {
		t.Fatalf("list: %d %q", code, out)
	} else if cfg := decode[config.Config](t, out); len(cfg.Sinks) != 0 {
		t.Fatalf("initial config: %+v", cfg)
	}
	if _, _, code := run("config", "set", "watch.interval", "1h"); code != 0 {
		t.Fatalf("set interval: %d", code)
	}
	if out, _, _ := run("config", "get", "watch.interval"); strings.TrimSpace(out) != `"1h"` {
		t.Fatalf("get interval: %q", out)
	}
	if _, _, code := run("config", "set", "sinks.phone.type", "ntfy"); code != 0 {
		t.Fatalf("set sink: %d", code)
	}
	if out, _, _ := run("config", "get", "sinks.phone"); !strings.Contains(out, "ntfy") {
		t.Fatalf("get sink: %q", out)
	}
	for _, args := range [][]string{
		{"config", "get", "nope"},
		{"config", "get", "sinks.phone.type.deeper"},
		{"config", "set", "colour", "red"},
		{"config", "set", "watch.interval", "abc"},
	} {
		if _, _, code := run(args...); code != 2 {
			t.Errorf("%v: %d", args, code)
		}
	}
}

func TestDoctor(t *testing.T) {
	isolate(t, site(t, map[string]string{searchPath: "search_atornillador.json"}))
	t.Setenv("HOME", t.TempDir())
	out, errOut, code := run("doctor")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	rep := decode[doctorReport](t, out)
	if !rep.OK || len(rep.Checks) < 4 {
		t.Fatalf("%+v", rep)
	}
	pretty, _, code := run("doctor", "--format", "pretty")
	if code != 0 || !strings.Contains(pretty, "ok") || !strings.Contains(pretty, "search") {
		t.Fatalf("%s", pretty)
	}

	isolate(t, "http://127.0.0.1:1")
	t.Setenv("HOME", t.TempDir())
	if _, errOut, code := run("doctor"); code != 1 || !strings.Contains(errOut, "doctor found problems") {
		t.Fatalf("broken site: exit %d %q", code, errOut)
	}
}

func TestSkills(t *testing.T) {
	isolate(t, "http://127.0.0.1:1")
	if out, _, code := run("skills", "list"); code != 0 || !strings.Contains(out, "lidl-usage") {
		t.Fatalf("list: %d %q", code, out)
	}
	if out, _, code := run("skills", "get", "lidl-usage"); code != 0 || !strings.Contains(out, "# lidl usage notes") {
		t.Fatalf("get: %d %q", code, out)
	}
	if _, _, code := run("skills", "get", "nope"); code != 2 {
		t.Fatalf("missing note: %d", code)
	}
}

func TestSinkListAndTest(t *testing.T) {
	isolate(t, "http://127.0.0.1:1")
	run("config", "set", "sinks.bad.type", "email")
	run("config", "set", "sinks.script.type", "exec")
	run("config", "set", "sinks.script.command", `["sh", "-c", "cat >/dev/null"]`)

	out, errOut, code := run("sink", "list")
	if code != 0 {
		t.Fatalf("list: %d %s", code, errOut)
	}
	rows := decode[[]sinkRow](t, out)
	if len(rows) != 2 {
		t.Fatalf("%+v", rows)
	}
	for _, r := range rows {
		if r.Name == "bad" && r.Problem == "" {
			t.Fatalf("a broken sink has no problem: %+v", r)
		}
	}
	if pretty, _, code := run("sink", "list", "--format", "pretty"); code != 0 || !strings.Contains(pretty, "SINK") {
		t.Fatalf("%s", pretty)
	}
	if out, errOut, code := run("sink", "test", "script"); code != 0 || !strings.Contains(out, `"delivered": true`) {
		t.Fatalf("test: %d %q %q", code, out, errOut)
	}
	if _, _, code := run("sink", "test", "nope"); code != 2 {
		t.Fatalf("unknown sink: %d", code)
	}
}

func TestMCPInstallAndServe(t *testing.T) {
	isolate(t, site(t, map[string]string{searchPath: "search_atornillador.json"}))
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	out, errOut, code := run("mcp", "install", "claude-code")
	if code != 0 {
		t.Fatalf("install: %d %s", code, errOut)
	}
	if !strings.Contains(out, `"action": "added"`) {
		t.Fatalf("%q", out)
	}
	if pretty, _, code := run("mcp", "install", "claude-code", "--format", "pretty"); code != 0 || !strings.Contains(pretty, "unchanged") {
		t.Fatalf("%s", pretty)
	}
	if _, _, code := run("mcp", "install", "vim"); code != 2 {
		t.Fatalf("unknown harness: %d", code)
	}

	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"search","arguments":{"keywords":"atornillador"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"product_show","arguments":{"product":"abc"}}}`,
	}, "\n")
	out, errOut, code = runWith(in, "mcp")
	if code != 0 {
		t.Fatalf("serve: %d %s", code, errOut)
	}
	if !strings.Contains(out, "protocolVersion") || !strings.Contains(out, "100405593") || !strings.Contains(out, `"isError":true`) {
		t.Fatalf("%s", out)
	}
}
