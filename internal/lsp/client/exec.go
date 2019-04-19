package client

import (
	"io"
	"log"
	"net"
	"os"
	"os/exec"

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

func StartServer(args []string, w io.Writer, rootdir string) (*Server, error) {
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
	lsp, err := New(p1, w, rootdir)
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

func DialServer(addr string, w io.Writer, rootdir string) (*Server, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	lsp, err := New(conn, w, rootdir)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to language server at %v", addr)
	}
	return &Server{
		cmd:      nil,
		protocol: conn,
		Conn:     lsp,
	}, nil
}
