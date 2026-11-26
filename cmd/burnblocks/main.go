// Command burnblocks reports Kimi Code and OpenRouter usage, credits, and
// estimated runway from the terminal.
//
// The command is read-only. It never sends inference requests, purchases
// credit, or creates, rotates, or revokes a credential. JSON goes to stdout and
// diagnostics go to stderr.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/acevedomiguel/burnblocks/kimi"
	"github.com/acevedomiguel/burnblocks/openrouter"
)

// version is replaced at build time with
// -ldflags "-X main.version=<version>".
var version = "dev"

// Exit codes are part of the CLI contract.
const (
	exitOK          = 0
	exitFailure     = 1
	exitUsage       = 2
	exitCredentials = 3
)

const usage = `burnblocks reports Kimi Code and OpenRouter usage.

Usage:
  burnblocks kimi [--json] [--config <path>] [--no-cache]
  burnblocks openrouter [--json] [--config <path>] [--no-cache]
  burnblocks --version
  burnblocks --help

Flags:
  --json            Emit one provider JSON snapshot on stdout
  --config <path>   Use an explicit configuration file
  --no-cache        Do not read or write the local cache

Each provider keeps its own versioned JSON schema and its own unit: Kimi reports
ratios and OpenRouter reports US dollars. Never total them together.

Exit codes:
  0 success   1 runtime or API failure   2 usage error   3 missing credentials
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	return runWithDeps(args, stdout, stderr, defaultDeps())
}

func runWithDeps(args []string, stdout, stderr io.Writer, d deps) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return exitUsage
	}

	switch args[0] {
	case "--version", "-v", "version":
		fmt.Fprintf(stdout, "burnblocks version %s\n", version)
		return exitOK
	case "--help", "-h", "help":
		printUsage(stdout)
		return exitOK
	case "kimi":
		return runKimi(args[1:], stdout, stderr, d)
	case "openrouter":
		return runOpenRouter(args[1:], stdout, stderr, d)
	default:
		fmt.Fprintf(stderr, "burnblocks: unknown command %q\n\n%s", args[0], usage)
		return exitUsage
	}
}

// usageError marks failures caused by the command line rather than the
// environment, so they exit 2 with an explanatory line.
type usageError struct{ msg string }

func (e *usageError) Error() string { return e.msg }

// options is the shared subcommand command line.
type options struct {
	jsonOutput bool
	configPath string
	noCache    bool
	// help records an explicit help request so it can exit 0 instead of being
	// reported as a usage error.
	help bool
}

// parseOptions parses one subcommand's flags. It rejects positional arguments
// so a typo is never silently ignored.
func parseOptions(name string, args []string) (options, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var opts options
	fs.BoolVar(&opts.jsonOutput, "json", false, "emit one JSON snapshot and exit")
	fs.StringVar(&opts.configPath, "config", "", "use an explicit configuration file")
	fs.BoolVar(&opts.noCache, "no-cache", false, "do not read or write the local cache")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return options{help: true}, nil
		}
		return options{}, &usageError{msg: err.Error()}
	}
	if fs.NArg() > 0 {
		return options{}, &usageError{msg: fmt.Sprintf("unexpected argument %q", fs.Arg(0))}
	}
	return opts, nil
}

// printUsage writes the usage block to w.
func printUsage(w io.Writer) { fmt.Fprint(w, usage) }

// deps carries the injectable seams the CLI needs. Production builds get real
// implementations; tests inject fakes so no test touches the keyring, the
// network, the cache, or the real user configuration.
type deps struct {
	// environment overrides the process environment for both configuration and
	// credential resolution. Production passes os.Environ as a map; nil would
	// make the credential store fall back to os.Getenv but would leave
	// configuration variables unread, so production never leaves it nil.
	environment map[string]string
	// kimiKeyring and openRouterKeyring are the OS secret stores. nil forces the
	// restricted-file fallback, which is how tests stay hermetic.
	kimiKeyring       kimi.Keyring
	openRouterKeyring openrouter.Keyring
	// kimiSource builds the one-shot Kimi usage source. Tests inject a fake.
	kimiSource func(baseURL, token, clientVersion string) (kimi.UsageSource, error)
	// openRouterAPI builds the read-only OpenRouter client. Tests inject a fake.
	openRouterAPI func(baseURL, management, apiKey, clientVersion string) (openrouter.API, error)
	// kimiState overrides the Kimi runway cache. nil selects the file store, or
	// a disabled store when --no-cache is set.
	kimiState kimi.StateStore
	// openRouterCache overrides the OpenRouter snapshot cache. nil selects the
	// file store, or no cache when --no-cache is set.
	openRouterCache *openrouter.CacheStore
	// kimiPaths and openRouterPaths resolve the configuration and cache layout.
	// Tests point them at a temporary directory so nothing touches the real user
	// layout.
	kimiPaths       func() (kimi.Paths, error)
	openRouterPaths func() (openrouter.Paths, error)
	// now supplies timestamps and cache ages.
	now func() time.Time
}

func defaultDeps() deps {
	return deps{
		environment:       environment(),
		kimiKeyring:       kimi.NewOSKeyring(),
		openRouterKeyring: openrouter.NewOSKeyring(),
		kimiSource: func(baseURL, token, clientVersion string) (kimi.UsageSource, error) {
			return kimi.NewSource(baseURL, token, kimi.WithVersion(clientVersion))
		},
		openRouterAPI: func(baseURL, management, apiKey, clientVersion string) (openrouter.API, error) {
			client, err := openrouter.NewClient(
				openrouter.WithBaseURL(baseURL),
				openrouter.WithManagementKey(management),
				openrouter.WithAPIKey(apiKey),
				openrouter.WithVersion(clientVersion),
			)
			if err != nil {
				return nil, err
			}
			return client, nil
		},
		kimiPaths:       kimi.DefaultPaths,
		openRouterPaths: openrouter.DefaultPaths,
		now:             time.Now,
	}
}

// environment snapshots the process environment as the engines expect it.
func environment() map[string]string {
	out := make(map[string]string, len(os.Environ()))
	for _, entry := range os.Environ() {
		for i := 0; i < len(entry); i++ {
			if entry[i] == '=' {
				out[entry[:i]] = entry[i+1:]
				break
			}
		}
	}
	return out
}

// reportUsageError prints a command-line error and the usage block.
func reportUsageError(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "burnblocks: %s\n\n%s", err.Error(), usage)
	return exitUsage
}

// configFilesFor returns the configuration files to merge, honoring an explicit
// flag over the platform default.
func configFilesFor(defaultPath, explicit string) []string {
	if explicit != "" {
		return []string{explicit}
	}
	return []string{defaultPath}
}
