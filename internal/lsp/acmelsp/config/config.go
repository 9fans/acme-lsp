package config

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"9fans.net/go/plan9/client"
	"github.com/fhs/acme-lsp/internal/lsp/protocol"
)

type Flags uint

const (
	LangServerFlags Flags = 1 << iota
	ProxyFlags
)

// File represents user configuration file for acme-lsp and L.
type File struct {
	// Network and address used for communication between acme-lsp and L
	ProxyNetwork, ProxyAddress string

	// Network and address where acme is serving 9P file server.
	AcmeNetwork, AcmeAddress string

	// Print more messages to stderr or log file
	Verbose bool

	// Write log to to this file instead of stderr
	LogFile string

	// Initial set of workspace directories
	WorkspaceDirectories []string

	// Root directory used for LSP initialization
	RootDirectory string

	// Format file when Put is executed in a window
	FormatOnSave bool

	// LSP code actions to run when Put is executed in a window.
	CodeActionsOnSave []protocol.CodeActionKind

	// LSP servers keyed by the language name
	LanguageServers map[string]LanguageServer
}

// Config configures acme-lsp and L.
type Config struct {
	File

	// Language servers supplied by -server or -dial flag
	LegacyLanguageServers []LegacyLanguageServer
}

// Language servers describes a LSP server.
type LanguageServer struct {
	// Command that runs the LSP server on stdin/stdout
	Command []string

	// Write LSP server stderr to this file
	LogFile string

	// Sever-specific LSP configuration
	Options interface{}
}

// LegacyLanguageServer describes a LSP server matching a filename.
type LegacyLanguageServer struct {
	// Regular expression pattern for filename
	Pattern string

	// Command that runs the LSP server on stdin/stdout
	Command []string

	// Network address of LSP server
	DialAddress string
}

func Default() *Config {
	return &Config{
		File: File{
			ProxyNetwork: "unix",
			ProxyAddress: filepath.Join(client.Namespace(), "acme-lsp.rpc"),
			AcmeNetwork:  "unix",
			AcmeAddress:  filepath.Join(client.Namespace(), "acme"),
		},
	}
}

func Load() (*Config, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	b, err := ioutil.ReadFile(filepath.Join(dir, "acme-lsp/config.json"))
	if err != nil {
		return nil, err
	}
	var f File
	err = json.Unmarshal(b, &f)
	if err != nil {
		return nil, err
	}
	return &Config{File: f}, nil
}

func ParseFlags(cfg *Config, flags Flags, f *flag.FlagSet, arguments []string) error {
	var (
		workspaces  string
		userServers serverFlag
		dialServers serverFlag
	)

	f.StringVar(&cfg.AcmeNetwork, "acme.net", cfg.AcmeNetwork,
		"network where acme is serving 9P file system")
	f.StringVar(&cfg.AcmeAddress, "acme.addr", cfg.AcmeAddress,
		"address where acme is serving 9P file system")

	if flags&ProxyFlags != 0 {
		f.StringVar(&cfg.ProxyNetwork, "proxy.net", cfg.ProxyNetwork,
			"network used for communication between acme-lsp and L")
		f.StringVar(&cfg.ProxyAddress, "proxy.addr", cfg.ProxyAddress,
			"address used for communication between acme-lsp and L")
	}
	if flags&LangServerFlags != 0 {
		f.BoolVar(&cfg.Verbose, "debug", false, "turn on debugging prints")
		f.StringVar(&workspaces, "workspaces", "", "colon-separated list of initial workspace directories")
		f.Var(&userServers, "server", `language server command for filename match (e.g. '\.go$:gopls')`)
		f.Var(&dialServers, "dial", `language server address for filename match (e.g. '\.go$:localhost:4389')`)
	}
	if err := f.Parse(arguments); err != nil {
		return err
	}

	if flags&LangServerFlags != 0 {
		if len(workspaces) > 0 {
			cfg.WorkspaceDirectories = strings.Split(workspaces, ":")
		}
		for _, sa := range userServers {
			cfg.LegacyLanguageServers = append(cfg.LegacyLanguageServers, LegacyLanguageServer{
				Pattern: sa.pattern,
				Command: strings.Fields(sa.args),
			})
		}
		for _, sa := range dialServers {
			cfg.LegacyLanguageServers = append(cfg.LegacyLanguageServers, LegacyLanguageServer{
				Pattern:     sa.pattern,
				DialAddress: sa.args,
			})
		}
	}
	return nil
}

type serverArgs struct {
	pattern, args string
}

type serverFlag []serverArgs

func (sf *serverFlag) String() string {
	return fmt.Sprintf("%v", []serverArgs(*sf))
}

func (sf *serverFlag) Set(val string) error {
	f := strings.SplitN(val, ":", 2)
	if len(f) != 2 {
		return errors.New("flag value must contain a colon")
	}
	if len(f[1]) == 0 {
		return errors.New("empty server command or addresss")
	}
	*sf = append(*sf, serverArgs{
		pattern: f[0],
		args:    f[1],
	})
	return nil
}
