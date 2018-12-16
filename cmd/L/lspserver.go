package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

type serverInfo struct {
	re   *regexp.Regexp
	args []string
	srv  *langServer
}

var serverList = []serverInfo{
	// golang.org/x/tools/cmd/golsp is not ready. It hasn't implmented
	// hover, references, and rename yet.
	//{regexp.MustCompile(`\.go$`), []string{"golsp"}, nil},
	{regexp.MustCompile(`\.go$`), []string{"go-langserver", "-gocodecompletion"}, nil},
	{regexp.MustCompile(`\.py$`), []string{"pyls"}, nil},
	{regexp.MustCompile(`\.c$`), []string{"cquery"}, nil},
}

func findServer(filename string) *serverInfo {
	for i, si := range serverList {
		if si.re.MatchString(filename) {
			return &serverList[i]
		}
	}
	return nil
}

type langServer struct {
	cmd  *exec.Cmd
	conn net.Conn
	lsp  *lspClient
}

func (s *langServer) Close() {
	if s != nil {
		s.lsp.Close()
		s.conn.Close()
	}
}

func startServer(args []string) (*langServer, error) {
	p0, p1 := net.Pipe()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = p0
	cmd.Stdout = p0
	if *debug {
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Start(); err != nil {
		return nil, errors.Wrapf(err, "failed to execute language server: %v")
	}
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("wait failed: %v\n", err)
		}
	}()
	lsp, err := newLSPClient(p1)
	if err != nil {
		cmd.Process.Kill()
		return nil, errors.Wrapf(err, "failed to connect to language server %q", args)
	}
	return &langServer{
		cmd:  cmd,
		conn: p1,
		lsp:  lsp,
	}, nil
}

func startServerForFile(filename string) (*langServer, error) {
	si := findServer(filename)
	if si == nil {
		return nil, errors.New(fmt.Sprintf("unknown language server for %v", filename))
	}
	if si.srv != nil {
		return si.srv, nil
	}
	srv, err := startServer(si.args)
	if err != nil {
		return nil, err
	}
	si.srv = srv
	return srv, nil
}

func closeServers() {
	for _, si := range serverList {
		si.srv.Close()
	}
}

func printServerList() {
	for _, si := range serverList {
		fmt.Printf("%v %v\n", si.re, strings.Join(si.args, " "))
	}
}
