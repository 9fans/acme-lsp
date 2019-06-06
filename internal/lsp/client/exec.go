package client

import (
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

	"github.com/pkg/errors"
)

type Server struct {
	cmd      *exec.Cmd
	protocol net.Conn
	Conn     *Conn
}

func (s *Server) Close() {
	if s != nil {
		s.Conn.Close()
		s.protocol.Close()
	}
}

func StartServer(args []string, cfg *Config) (*Server, error) {
	p0, p1 := net.Pipe()
	// TODO(fhs): use CommandContext?
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = p0
	cmd.Stdout = p0
	if Debug {
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Start(); err != nil {
		return nil, errors.Wrapf(err, "failed to execute language server")
	}
	go func() {
		// TODO(fhs): can we expose Wait and ask user to call it instead?
		if err := cmd.Wait(); err != nil {
			log.Printf("wait failed: %v\n", err)
		}
	}()
	lsp, err := New(p1, cfg)
	if err != nil {
		cmd.Process.Kill()
		return nil, errors.Wrapf(err, "failed to connect to language server %q", args)
	}
	return &Server{
		cmd:      cmd,
		protocol: p1,
		Conn:     lsp,
	}, nil
}

func DialServer(addr string, cfg *Config) (*Server, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	lsp, err := New(conn, cfg)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to language server at %v", addr)
	}
	return &Server{
		cmd:      nil,
		protocol: conn,
		Conn:     lsp,
	}, nil
}

// ServerInfo holds information about a LSP server and optionally a connection to it.
type ServerInfo struct {
	Re   *regexp.Regexp // filename regular expression
	Args []string       // LSP server command
	Addr string         // network address of LSP server
	srv  *Server        // running server instance
}

func (info *ServerInfo) start(cfg *Config) (*Server, error) {
	if info.srv != nil {
		return info.srv, nil
	}

	if len(info.Addr) > 0 {
		srv, err := DialServer(info.Addr, cfg)
		if err != nil {
			return nil, err
		}
		info.srv = srv
	} else {
		srv, err := StartServer(info.Args, cfg)
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
	workspaces map[string]struct{} // set of absolute paths to workspace directories
	cfg        Config
}

func NewServerSet(diagWriter DiagnosticsWriter) *ServerSet {
	return &ServerSet{
		Data:       nil,
		workspaces: make(map[string]struct{}),
		cfg: Config{
			w:          os.Stdout,
			diagWriter: diagWriter,
			rootdir:    "/",
			workspaces: nil,
		},
	}
}

func (ss *ServerSet) Register(pattern string, args []string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	info := &ServerInfo{
		Re:   re,
		Args: args,
	}
	ss.Data = append([]*ServerInfo{info}, ss.Data...)
	return nil
}

func (ss *ServerSet) RegisterDial(pattern string, addr string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	info := &ServerInfo{
		Re:   re,
		Addr: addr,
	}
	ss.Data = append([]*ServerInfo{info}, ss.Data...)
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

func (ss *ServerSet) CloseAll() {
	for _, info := range ss.Data {
		info.srv.Close()
	}
}

func (ss *ServerSet) PrintTo(w io.Writer) {
	for _, info := range ss.Data {
		if len(info.Addr) > 0 {
			fmt.Fprintf(w, "%v %v\n", info.Re, info.Addr)
		} else {
			fmt.Fprintf(w, "%v %v\n", info.Re, strings.Join(info.Args, " "))
		}
	}
}

func (ss *ServerSet) forEach(f func(*Conn) error) error {
	for _, info := range ss.Data {
		srv, err := info.start(&ss.cfg)
		if err != nil {
			return err
		}
		err = f(srv.Conn)
		if err != nil {
			return err
		}
	}
	return nil
}

// Workspaces returns a sorted list of current workspace directories.
func (ss *ServerSet) Workspaces() []string {
	var dirs []string
	for d := range ss.workspaces {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	return dirs
}

// InitWorkspaces initializes workspace directories.
func (ss *ServerSet) InitWorkspaces(dirs []string) error {
	dirs, err := AbsDirs(dirs)
	if err != nil {
		return err
	}
	ss.workspaces = make(map[string]struct{})
	for _, d := range dirs {
		ss.workspaces[d] = struct{}{}
	}
	// Update initial workspaces for servers not yet started.
	ss.cfg.workspaces = dirs
	return nil
}

// AddWorkspaces adds given workspace directories.
func (ss *ServerSet) AddWorkspaces(dirs []string) error {
	dirs, err := AbsDirs(dirs)
	if err != nil {
		return err
	}
	err = ss.forEach(func(conn *Conn) error {
		return conn.DidChangeWorkspaceFolders(dirs, nil)
	})
	if err != nil {
		return err
	}
	for _, d := range dirs {
		ss.workspaces[d] = struct{}{}
	}
	// Update initial workspaces for servers not yet started.
	ss.cfg.workspaces = ss.Workspaces()
	return nil
}

// RemoveWorkspaces removes given workspace directories.
func (ss *ServerSet) RemoveWorkspaces(dirs []string) error {
	dirs, err := AbsDirs(dirs)
	if err != nil {
		return err
	}
	err = ss.forEach(func(conn *Conn) error {
		return conn.DidChangeWorkspaceFolders(nil, dirs)
	})
	if err != nil {
		return err
	}
	for _, d := range dirs {
		delete(ss.workspaces, d)
	}
	// Update initial workspaces for servers not yet started.
	ss.cfg.workspaces = ss.Workspaces()
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
