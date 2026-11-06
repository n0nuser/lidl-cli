// Package cli wires cobra commands to the api client, the watch state and the
// output layer. One file per noun; handlers parse flags, call, print.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/n0nuser/lidl-cli/internal/api"
	"github.com/n0nuser/lidl-cli/internal/config"
	"github.com/n0nuser/lidl-cli/internal/output"
	"github.com/n0nuser/lidl-cli/internal/state"
)

// App is the state one invocation shares across commands.
type App struct {
	Version string
	Paths   config.Paths
	Cfg     config.Config
	Client  *api.Client
	Printer output.Printer

	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	// Interactive is true only when stdin, stdout and stderr are terminals and
	// --no-input is absent; prompts happen only then.
	Interactive bool

	state *state.DB

	flagFormat      string
	flagNoColor     bool
	flagErrorFormat string
	flagNoInput     bool
	flagDebug       bool
}

// Execute runs the CLI and returns the exit code.
func Execute(version string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	a := &App{Version: version, Stdin: stdin, Stdout: stdout, Stderr: stderr}
	root := a.rootCmd()
	root.SetArgs(args)
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cmd, err := root.ExecuteContextC(ctx)
	a.close()
	if err == nil {
		a.maybeUpdate(ctx, cmd)
		return 0
	}
	if errors.Is(err, context.Canceled) || ctx.Err() != nil {
		fmt.Fprintln(stderr, "lidl: interrupted")
		return 130
	}
	path := "lidl"
	if cmd != nil {
		path = cmd.CommandPath()
	}
	if isCobraUsage(err) {
		err = output.Usagef("%s. See `%s --help`", err.Error(), path)
	}
	jsonErrors := a.flagErrorFormat == "json" || (a.flagErrorFormat == "text" && os.Getenv("LIDL_ERROR_FORMAT") == "json")
	return output.PrintError(stderr, err, jsonErrors, version, path)
}

func (a *App) rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "lidl",
		Short: "Search lidl.es prices, offers and online availability from the terminal",
		Long: `Search lidl.es from the terminal: prices, offers, ratings and online availability.

Unofficial and read-only: no account, no cart, no orders. No login or browser
session is needed.

Output is JSON on stdout so every command pipes into jq; --format pretty is
for reading. Errors and notices go to stderr.

Examples:
  lidl search atornillador --format pretty
  lidl search atornillador --max-price 40 --sort price_asc --in-stock
  lidl product show 100405593 --format pretty
  lidl watch add product 100405593 --name taladro

Agent usage: lidl skills get lidl-usage`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       a.Version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return a.setup(cmd)
		},
	}
	root.SetVersionTemplate("lidl {{.Version}}\n")
	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return output.Usagef("%s. See `%s --help`", err.Error(), cmd.CommandPath())
	})
	pf := root.PersistentFlags()
	pf.StringVar(&a.flagFormat, "format", "json", "output format: json, jsonl, pretty, toon")
	pf.BoolVar(&a.flagNoColor, "no-color", false, "no colour in pretty output (also NO_COLOR)")
	pf.StringVar(&a.flagErrorFormat, "error-format", "text", "error rendering on stderr: text or json")
	pf.BoolVar(&a.flagNoInput, "no-input", false, "never prompt; fail when input is missing")
	pf.BoolVar(&a.flagDebug, "debug", false, "log every http request to stderr")
	_ = root.RegisterFlagCompletionFunc("format", cobra.FixedCompletions([]string{"json", "jsonl", "pretty", "toon"}, cobra.ShellCompDirectiveNoFileComp))
	_ = root.RegisterFlagCompletionFunc("error-format", cobra.FixedCompletions([]string{"text", "json"}, cobra.ShellCompDirectiveNoFileComp))

	root.AddCommand(
		a.searchCmd(), a.productCmd(), a.categoryCmd(),
		a.watchCmd(), a.sinkCmd(),
		a.mcpCmd(), a.configCmd(), a.doctorCmd(), a.skillsCmd(), a.updateCmd(),
	)
	return root
}

// setup runs before every command. It must stay cheap: no network.
func (a *App) setup(cmd *cobra.Command) error {
	if cmd.Name() == "help" || cmd.Name() == "completion" || (cmd.Parent() != nil && cmd.Parent().Name() == "completion") {
		return nil
	}
	format, err := output.ParseFormat(a.flagFormat)
	if err != nil {
		return err
	}
	if a.flagErrorFormat != "text" && a.flagErrorFormat != "json" {
		return output.Usagef("--error-format must be text or json")
	}
	stdout, _ := a.Stdout.(*os.File)
	stdin, _ := a.Stdin.(*os.File)
	stderr, _ := a.Stderr.(*os.File)
	a.Interactive = !a.flagNoInput && output.IsTerminal(stdin) && output.IsTerminal(stdout) && output.IsTerminal(stderr)
	a.Printer = output.Printer{W: a.Stdout, Format: format, Color: output.ColorEnabled(a.flagNoColor, stdout)}

	a.Paths = config.DefaultPaths()
	if a.Cfg, err = config.Load(a.Paths.ConfigFile); err != nil {
		return output.Usagef("%v. Fix it with `lidl config set` or by editing the file", err)
	}
	a.Client = api.New(a.Version)
	if a.flagDebug {
		a.Client.Debug = a.Stderr
	}
	return nil
}

// isCobraUsage recognises cobra's own argument errors, which arrive as
// plain errors: unknown command, wrong number of arguments, bad ValidArgs.
func isCobraUsage(err error) bool {
	msg := err.Error()
	for _, p := range []string{"unknown command", "accepts ", "requires at least", "requires at most", "invalid argument", "required flag"} {
		if strings.HasPrefix(msg, p) {
			return true
		}
	}
	return false
}

func (a *App) close() {
	if a.state != nil {
		a.state.Close()
		a.state = nil
	}
}

func (a *App) notice(format string, args ...any) {
	fmt.Fprintf(a.Stderr, "lidl: "+format+"\n", args...)
}
