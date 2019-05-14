package client

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/fhs/acme-lsp/internal/lsp/protocol"
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

func StartServer(args []string, w io.Writer, rootdir string, workspaces []string) (*Server, error) {
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
	lsp, err := New(p1, w, rootdir, workspaces)
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

func DialServer(addr string, w io.Writer, rootdir string, workspaces []string) (*Server, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	lsp, err := New(conn, w, rootdir, workspaces)
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
	re   *regexp.Regexp // filename regular expression
	args []string       // LSP server command
	addr string         // network address of LSP server
	srv  *Server        // running server instance
}

func (info *ServerInfo) start(workspaces []string) (*Server, error) {
	if info.srv != nil {
		return info.srv, nil
	}

	const rootdir = "/"

	if len(info.addr) > 0 {
		srv, err := DialServer(info.addr, os.Stdout, rootdir, workspaces)
		if err != nil {
			return nil, err
		}
		info.srv = srv
	} else {
		srv, err := StartServer(info.args, os.Stdout, rootdir, workspaces)
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
	Workspaces []string
}

func (ss *ServerSet) Register(pattern string, args []string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	info := &ServerInfo{
		re:   re,
		args: args,
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
		re:   re,
		addr: addr,
	}
	ss.Data = append([]*ServerInfo{info}, ss.Data...)
	return nil
}

func (ss *ServerSet) MatchFile(filename string) *ServerInfo {
	for i, info := range ss.Data {
		if info.re.MatchString(filename) {
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
	srv, err := info.start(ss.Workspaces)
	if err != nil {
		return nil, false, err
	}
	if false {
		// gopls doesn't support dynamic changes to workspace folders yet.
		// See https://github.com/golang/go/issues/31635
		fmt.Printf("server caps: %+v\n", srv.Conn.Capabilities)
		err = srv.Conn.DidChangeWorkspaceFolders([]protocol.WorkspaceFolder{
			{
				URI:  "file:///home/fhs/go/src/github.com/fhs/acme-lsp",
				Name: "/home/fhs/go/src/github.com/fhs/acme-lsp",
			},
		}, nil)
		if err != nil {
			return nil, false, err
		}
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
		if len(info.addr) > 0 {
			fmt.Fprintf(w, "%v %v\n", info.re, info.addr)
		} else {
			fmt.Fprintf(w, "%v %v\n", info.re, strings.Join(info.args, " "))
		}
	}
}
