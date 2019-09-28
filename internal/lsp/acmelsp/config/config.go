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
	Servers map[string]Server

	// Maping from LSP language ID to server key
	LanguageHandlers map[string]string

	// Servers determined by regular expression match on filename,
	// as supplied by -server and -dial flags.
	// This is deprecated in favor of LanguageHandlers.
	FilenameHandlers []FilenameHandler
}

// Config configures acme-lsp and L.
type Config struct {
	File

	// Show current configuration and exit
	ShowConfig bool
}

// Language servers describes a LSP server.
type Server struct {
	// Command that runs the LSP server on stdin/stdout
	Command []string

	// Dial address for server
	Address string

	// Write LSP server stderr to this file
	LogFile string

	// Sever-specific LSP configuration
	Options interface{}
}

// FilenameHandler contains a regular expression pattern that matches a filename
// and the associated server key.
type FilenameHandler struct {
	// Regular expression pattern for filename
	Pattern string

	// Server key
	ServerKey string
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
			Servers:          nil,
			LanguageHandlers: nil,
			FilenameHandlers: nil,
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
	for key := range cfg.Servers {
		if len(key) > 0 && key[0] == '_' {
			return nil, fmt.Errorf("server key %q begins with underscore", key)
		}
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
		if cfg.Servers == nil {
			cfg.Servers = make(map[string]Server)
		}
		handlers := make([]FilenameHandler, 0)
		for i, sa := range userServers {
			key := fmt.Sprintf("_userCmdServer%v", i)
			cfg.Servers[key] = Server{
				Command: strings.Fields(sa.args),
			}
			handlers = append(handlers, FilenameHandler{
				Pattern:   sa.pattern,
				ServerKey: key,
			})
		}
		for i, sa := range dialServers {
			key := fmt.Sprintf("_userDialServer%v", i)
			cfg.Servers[key] = Server{
				Address: sa.args,
			}
			handlers = append(handlers, FilenameHandler{
				Pattern:   sa.pattern,
				ServerKey: key,
			})
		}
		// Prepend to give higher priority to command line flags.
		cfg.FilenameHandlers = append(handlers, cfg.FilenameHandlers...)
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
