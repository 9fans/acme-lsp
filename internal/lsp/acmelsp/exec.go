package acmelsp

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/fhs/acme-lsp/internal/lsp"
	"github.com/fhs/acme-lsp/internal/lsp/acmelsp/config"
	"github.com/fhs/acme-lsp/internal/lsp/proxy"
)

type Server struct {
	conn   net.Conn
	Client *Client
}

func (s *Server) Close() {
	if s != nil {
		s.conn.Close()
	}
}

func execServer(cs *config.Server, cfg *ClientConfig) (*Server, error) {
	args := cs.Command

	stderr := os.Stderr
	if cs.StderrFile != "" {
		f, err := os.Create(cs.StderrFile)
		if err != nil {
			return nil, fmt.Errorf("could not create server StderrFile: %v", err)
		}
		stderr = f
	}

	startCommand := func() (*exec.Cmd, net.Conn, error) {
		p0, p1 := net.Pipe()
		// TODO(fhs): use CommandContext?
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdin = p0
		cmd.Stdout = p0
		if Verbose || cs.StderrFile != "" {
			cmd.Stderr = stderr
		}
		if err := cmd.Start(); err != nil {
			return nil, nil, fmt.Errorf("failed to execute language server: %v", err)
		}
		return cmd, p1, nil
	}
	cmd, p1, err := startCommand()
	if err != nil {
		return nil, err
	}
	srv := &Server{
		conn: p1,
	}

	// Restart server if it dies.
	go func() {
		for {
			err := cmd.Wait()
			log.Printf("language server %v exited: %v; restarting...", args[0], err)

			// TODO(fhs): cancel using context?
			srv.conn.Close()

			cmd, p1, err = startCommand()
			if err != nil {
				log.Printf("%v", err)
				return
			}
			srv.conn = p1

			go func() {
				// Reinitialize existing client instead of creating a new one
				// because it's still being used.
				if err := srv.Client.init(p1, cfg); err != nil {
					log.Printf("initialize after server restart failed: %v", err)
					cmd.Process.Kill()
					srv.conn.Close()
				}
			}()
		}
	}()

	srv.Client, err = NewClient(p1, cfg)
	if err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("failed to connect to language server %q: %v", args, err)
	}
	return srv, nil
}

func dialServer(cs *config.Server, cfg *ClientConfig) (*Server, error) {
	conn, err := net.Dial("tcp", cs.Address)
	if err != nil {
		return nil, err
	}
	c, err := NewClient(conn, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to language server at %v: %v", cs.Address, err)
	}
	return &Server{
		conn:   conn,
		Client: c,
	}, nil
}

// ServerInfo holds information about a LSP server and optionally a connection to it.
type ServerInfo struct {
	*config.Server
	*config.FilenameHandler

	Re     *regexp.Regexp // filename regular expression
	Logger *log.Logger    // Logger for config.Server.LogFile
	srv    *Server        // running server instance
}

func (info *ServerInfo) start(cfg *ClientConfig) (*Server, error) {
	if info.srv != nil {
		return info.srv, nil
	}

	if len(info.Address) > 0 {
		srv, err := dialServer(info.Server, cfg)
		if err != nil {
			return nil, err
		}
		info.srv = srv
	} else {
		srv, err := execServer(info.Server, cfg)
		if err != nil {
			return nil, err
		}
		info.srv = srv
	}
	return info.srv, nil
}

// ServerSet holds information about a set of LSP servers and connection to them,
// which are created on-demand.
type ServerSet struct {
	Data       []*ServerInfo
	diagWriter DiagnosticsWriter
	workspaces map[protocol.DocumentURI]*protocol.WorkspaceFolder // set of workspace folders
	cfg        *config.Config
}

// NewServerSet creates a new server set from config.
func NewServerSet(cfg *config.Config, diagWriter DiagnosticsWriter) (*ServerSet, error) {
	workspaces := make(map[protocol.DocumentURI]*protocol.WorkspaceFolder)
	if len(cfg.WorkspaceDirectories) > 0 {
		folders, err := lsp.DirsToWorkspaceFolders(cfg.WorkspaceDirectories)
		if err != nil {
			return nil, err
		}
		for i := range folders {
			d := &folders[i]
			workspaces[d.URI] = d
		}
	}

	var data []*ServerInfo
	for i, h := range cfg.FilenameHandlers {
		cs, ok := cfg.Servers[h.ServerKey]
		if !ok {
			return nil, fmt.Errorf("server not found for key %q", h.ServerKey)
		}
		if len(cs.Command) == 0 && len(cs.Address) == 0 {
			return nil, fmt.Errorf("invalid server for key %q", h.ServerKey)
		}
		re, err := regexp.Compile(h.Pattern)
		if err != nil {
			return nil, err
		}
		var logger *log.Logger
		if cs.LogFile != "" {
			f, err := os.Create(cs.LogFile)
			if err != nil {
				return nil, fmt.Errorf("could not create server %v LogFile: %v", h.ServerKey, err)
			}
			logger = log.New(f, "", log.LstdFlags)
		}
		data = append(data, &ServerInfo{
			Server:          cs,
			FilenameHandler: &cfg.FilenameHandlers[i],
			Re:              re,
			Logger:          logger,
		})
	}
	return &ServerSet{
		Data:       data,
		diagWriter: diagWriter,
		workspaces: workspaces,
		cfg:        cfg,
	}, nil
}

func (ss *ServerSet) MatchFile(filename string) *ServerInfo {
	for i, info := range ss.Data {
		if info.Re.MatchString(filename) {
			return ss.Data[i]
		}
	}
	return nil
}

func (ss *ServerSet) ClientConfig(info *ServerInfo) *ClientConfig {
	return &ClientConfig{
		Server:          info.Server,
		FilenameHandler: info.FilenameHandler,
		RootDirectory:   ss.cfg.RootDirectory,
		HideDiag:        ss.cfg.HideDiagnostics,
		RPCTrace:        ss.cfg.RPCTrace,
		DiagWriter:      ss.diagWriter,
		Workspaces:      ss.Workspaces(),
		Logger:          info.Logger,
	}
}

func (ss *ServerSet) StartForFile(filename string) (*Server, bool, error) {
	info := ss.MatchFile(filename)
	if info == nil {
		return nil, false, nil // unknown language server
	}
	srv, err := info.start(ss.ClientConfig(info))
	if err != nil {
		return nil, false, err
	}
	return srv, true, err
}

func (ss *ServerSet) ServerMatch(ctx context.Context, filename string) (proxy.Server, bool, error) {
	srv, found, err := ss.StartForFile(filename)
	if err != nil || !found {
		return nil, found, err
	}
	return srv.Client, found, err
}

func (ss *ServerSet) CloseAll() {
	for _, info := range ss.Data {
		info.srv.Close()
	}
}

func (ss *ServerSet) PrintTo(w io.Writer) {
	for _, info := range ss.Data {
		if len(info.Address) > 0 {
			fmt.Fprintf(w, "%v %v\n", info.Re, info.Address)
		} else {
			fmt.Fprintf(w, "%v %v\n", info.Re, strings.Join(info.Command, " "))
		}
	}
}

func (ss *ServerSet) forEach(f func(*Client) error) error {
	for _, info := range ss.Data {
		srv, err := info.start(ss.ClientConfig(info))
		if err != nil {
			return err
		}
		err = f(srv.Client)
		if err != nil {
			return err
		}
	}
	return nil
}

// Workspaces returns a sorted list of current workspace directories.
func (ss *ServerSet) Workspaces() []protocol.WorkspaceFolder {
	var folders []protocol.WorkspaceFolder
	for i := range ss.workspaces {
		folders = append(folders, *ss.workspaces[i])
	}
	sort.Slice(folders, func(i, j int) bool {
		return folders[i].URI < folders[j].URI
	})
	return folders
}

// DidChangeWorkspaceFolders adds and removes given workspace folders.
func (ss *ServerSet) DidChangeWorkspaceFolders(ctx context.Context, added, removed []protocol.WorkspaceFolder) error {
	err := ss.forEach(func(c *Client) error {
		return c.DidChangeWorkspaceFolders(ctx, &protocol.DidChangeWorkspaceFoldersParams{
			Event: protocol.WorkspaceFoldersChangeEvent{
				Added:   added,
				Removed: removed,
			},
		})
	})
	if err != nil {
		return err
	}
	for i := range added {
		d := &added[i]
		ss.workspaces[d.URI] = d
	}
	for _, d := range removed {
		delete(ss.workspaces, d.URI)
	}
	return nil
}

// AbsDirs returns the absolute representation of directories dirs.
func AbsDirs(dirs []string) ([]string, error) {
	a := make([]string, len(dirs))
	for i, d := range dirs {
		d, err := filepath.Abs(d)
		if err != nil {
			return nil, err
		}
		a[i] = d
	}
	return a, nil
}
