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
	lang string
	args []string
	srv  *langServer
}

var serverList = []serverInfo{
	// golang.org/x/tools/cmd/golsp is not ready. It hasn't implmented
	// hover, references, and rename yet.
	//{regexp.MustCompile(`\.go$`), "go",{"golsp"}, nil},
	{regexp.MustCompile(`\.go$`), "go", []string{"go-langserver", "-gocodecompletion"}, nil},
	{regexp.MustCompile(`\.py$`), "python", []string{"pyls"}, nil},
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

func startServers() {
	langDone := make(map[string]bool, len(serverList))

	for i, si := range serverList {
		if langDone[si.lang] {
			continue
		}
		s, err := startServer(si.lang, si.args)
		if err != nil {
			log.Printf("cound not start %v server: %v\n", si.lang, err)
			continue
		}
		serverList[i].srv = s
		langDone[si.lang] = true
	}
}

func startServer(lang string, args []string) (*langServer, error) {
	p0, p1 := net.Pipe()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = p0
	cmd.Stdout = p0
	if *debug {
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Start(); err != nil {
		return nil, errors.Wrapf(err, "failed to execute %v server: %v\n", lang, err)
	}
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("%v server failed: %v\n", lang, err)
		}
	}()
	lsp, err := newLSPClient(p1)
	if err != nil {
		cmd.Process.Kill()
		return nil, errors.Wrapf(err, "failed to connect to %v server: %v\n", lang, err)
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
	srv, err := startServer(si.lang, si.args)
	if err != nil {
		return nil, err
	}
	si.srv = srv
	return srv, nil
}

func killServers() {
	for _, si := range serverList {
		si.srv.Close()
	}
}

func printServerList() {
	for _, si := range serverList {
		fmt.Printf("%v %v\n", si.re, strings.Join(si.args, " "))
	}
}
