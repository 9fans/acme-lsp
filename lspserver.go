package main

import (
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
)

var serverCommands = map[string][]string{
	"go":     {"go-langserver"},
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

func startServers() {
	for lang, args := range serverCommands {
		p0, p1 := net.Pipe()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdin = p0
		cmd.Stdout = p0
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			log.Printf("failed to start %v language server: %v\n", lang, err)
			continue
		}
		go func() {
			if err := cmd.Wait(); err != nil {
				log.Fatal(err)
			}
		}()
		lsp, err := newLSPClient(p1)
		if err != nil {
			cmd.Process.Kill()
			log.Printf("failed to connect to %v server: %v\n", lang, err)
			continue
		}
		servers[lang] = &langServer{
			cmd:  cmd,
			conn: p1,
			lsp:  lsp,
		}
	}
}

func stopServers() {
	for _, c := range servers {
		c.lsp.Close()
		c.conn.Close()
		c.cmd.Process.Kill()
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
