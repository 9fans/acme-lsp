package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"9fans.net/go/plan9/client"
	"github.com/BurntSushi/toml"
)

type Flags uint

const (
	LangServerFlags Flags = 1 << iota
	ProxyFlags
	ShowConfigFlag
)

// File represents user configuration file for acme-lsp and L.
type File struct {
	// Network and address used for communication between acme-lsp and L
	ProxyNetwork, ProxyAddress string

	// Network and address where acme is serving 9P file server.
	AcmeNetwork, AcmeAddress string

	// Print more messages to stderr or log file
	Verbose bool

	// Initial set of workspace directories
	WorkspaceDirectories []string

	// Root directory used for LSP initialization
	RootDirectory string

	// Format file when Put is executed in a window
	FormatOnPut bool

	// LSP code actions to run when Put is executed in a window.
	//CodeActionsOnSave []protocol.CodeActionKind

	// Write log to to this file instead of stderr
	//LogFile string

	// LSP servers keyed by a user provided name
	//LanguageServers map[string]LanguageServer

	// Maping from LSP language ID to server in LanguageServers
	//LanguageHandlers map[string]string
}

// Config configures acme-lsp and L.
type Config struct {
	File

	// Show current configuration and exit
	ShowConfig bool

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
	rootDir := "/"
	switch runtime.GOOS {
	case "windows":
		rootDir = `C:\`
	}
	return &Config{
		File: File{
			ProxyNetwork:         "unix",
			ProxyAddress:         filepath.Join(client.Namespace(), "acme-lsp.rpc"),
			AcmeNetwork:          "unix",
			AcmeAddress:          filepath.Join(client.Namespace(), "acme"),
			Verbose:              false,
			WorkspaceDirectories: nil,
			RootDirectory:        rootDir,
			FormatOnPut:          true,
			//CodeActionsOnSave: []protocol.CodeActionKind{
			//	protocol.SourceOrganizeImports,
			//},
			//LanguageServers: map[string]LanguageServer{
			//	"gopls": LanguageServer{
			//		Command: []string{"gopls", "serve"},
			//	},
			//	"gopls-debug": LanguageServer{
			//		Command: []string{"gopls", "serve", "-debug=localhost:6060"},
			//	},
			//	"pyls": LanguageServer{
			//		Command: []string{"pyls"},
			//	},
			//},
			//LanguageHandlers: map[string]string{
			//	"go":     "gopls",
			//	"go.mod": "gopls",
			//	"py":     "pyls",
			//},
		},
	}
}

func userConfigFilename() (string, error) {
	dir, err := UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "acme-lsp/config.toml"), nil
}

func Load() (*Config, error) {
	def := Default()

	filename, err := userConfigFilename()
	if err != nil {
		return nil, err
	}
	_, err = os.Stat(filename)
	if os.IsNotExist(err) {
		return def, nil
	}

	cfg, err := load(filename)
	if err != nil {
		return nil, err
	}

	if cfg.File.ProxyNetwork == "" {
		cfg.File.ProxyNetwork = def.File.ProxyNetwork
	}
	if cfg.File.ProxyAddress == "" {
		cfg.File.ProxyAddress = def.File.ProxyAddress
	}
	if cfg.File.AcmeNetwork == "" {
		cfg.File.AcmeNetwork = def.File.AcmeNetwork
	}
	if cfg.File.AcmeAddress == "" {
		cfg.File.AcmeAddress = def.File.AcmeAddress
	}
	if cfg.File.RootDirectory == "" {
		cfg.File.RootDirectory = def.File.RootDirectory
	}
	return cfg, nil
}

func load(filename string) (*Config, error) {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var f File
	err = toml.Unmarshal(b, &f)
	if err != nil {
		return nil, err
	}
	return &Config{File: f}, nil
}

func Show(w io.Writer, cfg *Config) error {
	filename, err := userConfigFilename()
	if err == nil {
		fmt.Fprintf(w, "# Configuration file location: %v\n\n", filename)
	} else {
		fmt.Fprintf(w, "# Cound not find configuration file location: %v\n\n", err)
	}
	return toml.NewEncoder(w).Encode(cfg.File)
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
		f.BoolVar(&cfg.Verbose, "debug", cfg.Verbose, "turn on debugging prints")
		f.StringVar(&workspaces, "workspaces", "", "colon-separated list of initial workspace directories")
		f.Var(&userServers, "server", `language server command for filename match (e.g. '\.go$:gopls')`)
		f.Var(&dialServers, "dial", `language server address for filename match (e.g. '\.go$:localhost:4389')`)
	}
	if flags&ShowConfigFlag != 0 {
		f.BoolVar(&cfg.ShowConfig, "showconfig", false, "show configuration values and exit")
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
