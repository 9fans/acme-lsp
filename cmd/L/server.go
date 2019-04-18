package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/fhs/acme-lsp/internal/lsp/client"
)

type serverInfo struct {
	re   *regexp.Regexp
	args []string
	addr string
	srv  *client.Server
}

func (si *serverInfo) Connect() (*client.Server, error) {
	if len(si.addr) > 0 {
		return client.DialServer(si.addr, os.Stdout, *rootdir)
	}
	return client.StartServer(si.args, os.Stdout, *rootdir)
}

var serverList = []serverInfo{
	// golang.org/x/tools/cmd/gopls is not ready. It hasn't implmented
	// references, and rename yet.
	//{regexp.MustCompile(`\.go$`), []string{"gopls"}, "", nil},
	{regexp.MustCompile(`\.go$`), []string{"go-langserver", "-gocodecompletion"}, "", nil},
	{regexp.MustCompile(`\.py$`), []string{"pyls"}, "", nil},
	//{regexp.MustCompile(`\.c$`), []string{"cquery"}, "", nil},
}

func findServer(filename string) *serverInfo {
	for i, si := range serverList {
		if si.re.MatchString(filename) {
			return &serverList[i]
		}
	}
	return nil
}

func startServerForFile(filename string) (*client.Server, error) {
	si := findServer(filename)
	if si == nil {
		return nil, fmt.Errorf("unknown language server for %v", filename)
	}
	if si.srv == nil {
		srv, err := si.Connect()
		if err != nil {
			return nil, err
		}
		si.srv = srv
	}
	return si.srv, nil
}

func closeServers() {
	for _, si := range serverList {
		si.srv.Close()
	}
}

func printServerList() {
	for _, si := range serverList {
		if len(si.addr) > 0 {
			fmt.Printf("%v %v\n", si.re, si.addr)
		} else {
			fmt.Printf("%v %v\n", si.re, strings.Join(si.args, " "))
		}
	}
}
