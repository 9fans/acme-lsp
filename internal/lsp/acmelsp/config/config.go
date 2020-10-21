// Package config defines LSP tools configuration.
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

	"github.com/BurntSushi/toml"
	"github.com/fhs/9fans-go/plan9/client"
	"github.com/fhs/acme-lsp/internal/lsp/protocol"
)

// Flags represent a set of command line flags.
type Flags uint

const (
	// LangServerFlags include flags that configure LSP servers.
	LangServerFlags Flags = 1 << iota

	// ProxyFlags include flags that configure how to connect to proxy server.
	ProxyFlags
)

// File represents user configuration file for acme-lsp and L.
type File struct {
	// Network and address used for communication between acme-lsp and L.
	// Only required on systems without unix domain socket.
	ProxyNetwork, ProxyAddress string

	// Network and address where acme is serving 9P file server.
	// Only required on systems without unix domain socket.
	AcmeNetwork, AcmeAddress string

	// Initial set of workspace directories.
	WorkspaceDirectories []string

	// Root directory used for LSP initialization.
	RootDirectory string

	// Don't show diagnostics sent by the LSP server.
	HideDiagnostics bool

	// Format file when Put is executed in a window.
	FormatOnPut bool

	// Print to stderr the full rpc trace in lsp inspector format
	RPCTrace bool

	// LSP code actions to run when Put is executed in a window.
	CodeActionsOnPut []protocol.CodeActionKind

	// LSP servers keyed by a user provided name.
	Servers map[string]*Server

	// Servers determined by regular expression match on filename,
	// as supplied by -server and -dial flags.
	FilenameHandlers []FilenameHandler
}

// Config configures acme-lsp and L.
type Config struct {
	File

	// Show current configuration and exit
	ShowConfig bool

	// Print more messages to stderr
	Verbose bool
}

// Server describes a LSP server.
type Server struct {
	// Command that speaks LSP on stdin/stdout.
	// Can be empty if Address is given.
	Command []string

	// Dial address for LSP server. Ignored if Command is not empty.
	Address string

	// Write stderr of Command to this file.
	// If it's not an absolute path, it'll become relative to the cache directory.
	StderrFile string

	// Write log messages (window/logMessage notifications) sent by LSP server
	// to this file instead of stderr.
	// If it's not an absolute path, it'll become relative to the cache directory.
	LogFile string

	// Options contain server-specific settings that are passed as-is to the LSP server.
	Options interface{}
}

// FilenameHandler contains a regular expression pattern that matches a filename
// and the associated server key.
type FilenameHandler struct {
	// Pattern is a regular expression that matches filename.
	Pattern string

	// Language identifier (e.g. "go" or "python")
	// See list of languages here:
	// https://microsoft.github.io/language-server-protocol/specifications/specification-current/#textDocumentItem
	LanguageID string

	// ServerKey is the key in Config.File.Servers.
	ServerKey string
}

// Default returns the default Config.
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
			WorkspaceDirectories: nil,
			RootDirectory:        rootDir,
			FormatOnPut:          true,
			CodeActionsOnPut: []protocol.CodeActionKind{
				protocol.SourceOrganizeImports,
			},
			Servers:          nil,
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

// Load loads Config from file system, falling back to a default if it doesn't exist.
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
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	cacheDir = filepath.Join(cacheDir, "acme-lsp")
	err = os.MkdirAll(cacheDir, 0700)
	if err != nil {
		return nil, err
	}
	for key := range cfg.Servers {
		if len(key) > 0 && key[0] == '_' {
			return nil, fmt.Errorf("server key %q begins with underscore", key)
		}
		s := cfg.File.Servers[key]
		if s.StderrFile != "" && !filepath.IsAbs(s.StderrFile) {
			s.StderrFile = filepath.Join(cacheDir, s.StderrFile)
		}
		if s.LogFile != "" && !filepath.IsAbs(s.LogFile) {
			s.LogFile = filepath.Join(cacheDir, s.LogFile)
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

// Write writes Config to writer w.
func Write(w io.Writer, cfg *Config) error {
	filename, err := userConfigFilename()
	if err == nil {
		fmt.Fprintf(w, "# Configuration file location: %v\n\n", filename)
	} else {
		fmt.Fprintf(w, "# Cound not find configuration file location: %v\n\n", err)
	}
	return toml.NewEncoder(w).Encode(cfg.File)
}

// ParseFlags parses command line flags and updates Config.
func (cfg *Config) ParseFlags(flags Flags, f *flag.FlagSet, arguments []string) error {
	var (
		workspaces  string
		userServers serverFlag
		dialServers serverFlag
	)

	f.StringVar(&cfg.AcmeNetwork, "acme.net", cfg.AcmeNetwork,
		"network where acme is serving 9P file system")
	f.StringVar(&cfg.AcmeAddress, "acme.addr", cfg.AcmeAddress,
		"address where acme is serving 9P file system")
	f.BoolVar(&cfg.Verbose, "v", cfg.Verbose, "Verbose output")
	f.BoolVar(&cfg.ShowConfig, "showconfig", false, "show configuration values and exit")

	if flags&ProxyFlags != 0 {
		f.StringVar(&cfg.ProxyNetwork, "proxy.net", cfg.ProxyNetwork,
			"network used for communication between acme-lsp and L")
		f.StringVar(&cfg.ProxyAddress, "proxy.addr", cfg.ProxyAddress,
			"address used for communication between acme-lsp and L")
	}
	if flags&LangServerFlags != 0 {
		f.BoolVar(&cfg.Verbose, "debug", cfg.Verbose, "turn on debugging prints (deprecated: use -v)")
		f.StringVar(&cfg.RootDirectory, "rootdir", cfg.RootDirectory, "root directory used for LSP initialization")
		f.BoolVar(&cfg.HideDiagnostics, "hidediag", false, "hide diagnostics sent by LSP server")
		f.BoolVar(&cfg.RPCTrace, "rpc.trace", false, "print the full rpc trace in lsp inspector format")
		f.StringVar(&workspaces, "workspaces", "", "colon-separated list of initial workspace directories")
		f.Var(&userServers, "server", `map filename to language server command. The format is
'handlers:cmd' where cmd is the LSP server command and handlers is
a comma separated list of 'regexp[@lang]'. The regexp matches the
filename and lang is a language identifier. (e.g. '\.go$:gopls' or
'go.mod$@go.mod,go.sum$@go.sum,\.go$@go:gopls')`)
		f.Var(&dialServers, "dial", `map filename to language server address. The format is
'handlers:host:port'. See -server flag for format of
handlers. (e.g. '\.go$:localhost:4389')`)
	}
	if err := f.Parse(arguments); err != nil {
		return err
	}

	if flags&LangServerFlags != 0 {
		if len(workspaces) > 0 {
			cfg.WorkspaceDirectories = strings.Split(workspaces, ":")
		}
		if cfg.Servers == nil {
			cfg.Servers = make(map[string]*Server)
		}
		handlers := make([]FilenameHandler, 0)
		for i, sa := range userServers {
			key := fmt.Sprintf("_userCmdServer%v", i)
			cfg.Servers[key] = &Server{
				Command: strings.Fields(sa.args),
			}
			for _, h := range sa.handlers {
				h.ServerKey = key
				handlers = append(handlers, h)
			}
		}
		for i, sa := range dialServers {
			key := fmt.Sprintf("_userDialServer%v", i)
			cfg.Servers[key] = &Server{
				Address: sa.args,
			}
			for _, h := range sa.handlers {
				h.ServerKey = key
				handlers = append(handlers, h)
			}
		}
		// Prepend to give higher priority to command line flags.
		cfg.FilenameHandlers = append(handlers, cfg.FilenameHandlers...)
	}
	return nil
}

type serverArgs struct {
	handlers []FilenameHandler
	args     string
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
	// allow f[0] to be empty, as that's a valid regexp that matches anything
	if len(f[1]) == 0 {
		return errors.New("empty server command or addresss")
	}
	pairs := strings.Split(f[0], ",")
	args := f[1]

	var handlers []FilenameHandler
	for _, pp := range pairs {
		f := strings.SplitN(pp, "@", 2)
		if len(f) != 2 {
			handlers = append(handlers, FilenameHandler{
				Pattern:    pp,
				LanguageID: "",
			})
		} else {
			handlers = append(handlers, FilenameHandler{
				Pattern:    f[0],
				LanguageID: f[1],
			})
		}
	}
	*sf = append(*sf, serverArgs{
		handlers: handlers,
		args:     args,
	})
	return nil
}
