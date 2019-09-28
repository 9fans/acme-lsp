package lsp

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

	"github.com/fhs/acme-lsp/internal/lsp/acmelsp/config"
	"github.com/fhs/acme-lsp/internal/lsp/protocol"
	"github.com/fhs/acme-lsp/internal/lsp/proxy"
	"github.com/pkg/errors"
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

func StartServer(args []string, cfg *Config) (*Server, error) {
	startCommand := func() (*exec.Cmd, net.Conn, error) {
		p0, p1 := net.Pipe()
		// TODO(fhs): use CommandContext?
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdin = p0
		cmd.Stdout = p0
		if Debug {
			cmd.Stderr = os.Stderr
		}
		if err := cmd.Start(); err != nil {
			return nil, nil, errors.Wrapf(err, "failed to execute language server")
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
		return nil, errors.Wrapf(err, "failed to connect to language server %q", args)
	}
	return srv, nil
}

func DialServer(addr string, cfg *Config) (*Server, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	c, err := NewClient(conn, cfg)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to language server at %v", addr)
	}
	return &Server{
		conn:   conn,
		Client: c,
	}, nil
}

// ServerInfo holds information about a LSP server and optionally a connection to it.
type ServerInfo struct {
	*config.Server

	Re  *regexp.Regexp // filename regular expression
	srv *Server        // running server instance
}

func (info *ServerInfo) start(cfg *Config) (*Server, error) {
	if info.srv != nil {
		return info.srv, nil
	}

	if len(info.Address) > 0 {
		srv, err := DialServer(info.Address, cfg)
		if err != nil {
			return nil, err
		}
		info.srv = srv
	} else {
		srv, err := StartServer(info.Command, cfg)
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
	workspaces map[protocol.DocumentURI]*protocol.WorkspaceFolder // set of workspace folders
	cfg        Config
}

func NewServerSet(cfg *Config) *ServerSet {
	return &ServerSet{
		Data:       nil,
		workspaces: make(map[protocol.DocumentURI]*protocol.WorkspaceFolder),
		cfg:        *cfg,
	}
}

func (ss *ServerSet) Register(pattern string, cs *config.Server) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	info := &ServerInfo{
		Server: cs,
		Re:     re,
	}
	ss.Data = append(ss.Data, info)
	return nil
}

func (ss *ServerSet) MatchFile(filename string) *ServerInfo {
	for i, info := range ss.Data {
		if info.Re.MatchString(filename) {
			return ss.Data[i]
		}
	}
	return nil
}

func (ss *ServerSet) StartForFile(filename string) (*Server, bool, error) {
	info := ss.MatchFile(filename)
	if info == nil {
		return nil, false, nil // unknown language server
	}
	srv, err := info.start(&ss.cfg)
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
		srv, err := info.start(&ss.cfg)
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

// InitWorkspaces initializes workspace directories.
func (ss *ServerSet) InitWorkspaces(folders []protocol.WorkspaceFolder) error {
	ss.workspaces = make(map[protocol.DocumentURI]*protocol.WorkspaceFolder)
	for i := range folders {
		d := &folders[i]
		ss.workspaces[d.URI] = d
	}
	// Update initial workspaces for servers not yet started.
	ss.cfg.Workspaces = folders
	return nil
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
	// Update initial workspaces for servers not yet started.
	// TODO(fhs): do we need a lock here?
	ss.cfg.Workspaces = ss.Workspaces()
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
