package main

import (
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"
)

var serverCommands = map[string][]string{
	// golang.org/x/tools/cmd/golsp is not ready. It hasn't implmented
	// hover, references, and rename yet.
	//"go": {"golsp"},
	"go":     {"go-langserver", "-gocodecompletion"},
	"python": {"pyls"},
}

func lspLang(filename string) string {
	ext := filepath.Ext(filename)
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	}
	return ""
}

var servers = make(map[string]*langServer, 0)

type langServer struct {
	cmd  *exec.Cmd
	conn net.Conn
	lsp  *lspClient
}

func (s *langServer) Kill() {
	s.lsp.Close()
	s.conn.Close()
	s.cmd.Process.Kill()
}

func startServers() {
	for lang, args := range serverCommands {
		s, err := startServer(lang, args)
		if err != nil {
			log.Printf("cound not start %v server: %v\n", lang, err)
			continue
		}
		servers[lang] = s
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
	lang := lspLang(filename)
	cmd, ok := serverCommands[lang]
	if !ok {
		return nil, errors.New("unknown language " + lang)
	}
	return startServer(lang, cmd)
}

func killServers() {
	for _, c := range servers {
		c.Kill()
	}
}

func runServer() net.Conn {
	p0, p1 := net.Pipe()
	cmd := exec.Command("go-langserver", "-gocodecompletion")
	cmd.Stdin = p0
	cmd.Stdout = p0
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Fatal(err)
		}
	}()
	return p1
}

func dialServer() net.Conn {
	conn, err := net.Dial("tcp", "localhost:4389")
	if err != nil {
		log.Fatal(err)
	}
	return conn
}
