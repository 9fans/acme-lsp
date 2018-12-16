package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"9fans.net/go/acme"
	"github.com/pkg/errors"
)

var debug = flag.Bool("debug", false, "turn on debugging prints")
var extraServers serverFlag

func usage() {
	fmt.Fprintf(os.Stderr, "usage: %v <command> [args...]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\n")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "Run \"go doc github.com/fhs/acme-lsp/cmd/L\" for more detailed usage help.\n")
	os.Exit(2)
}

func main() {
	flag.Usage = usage
	flag.Var(&extraServers, "server", `set language server for regex match (e.g. '\.go$:golsp')`)
	flag.Parse()

	if len(extraServers) > 0 {
		// give priority to user-defined servers
		serverList = append(extraServers, serverList...)
	}
	if flag.NArg() < 1 {
		usage()
	}
	switch flag.Arg(0) {
	case "win":
		if flag.NArg() < 2 {
			usage()
		}
		watch(flag.Arg(1))
		closeServers()
		return

	case "monitor":
		monitor()
		closeServers()
		return

	case "servers":
		printServerList()
		return
	}
	w, err := openCurrentWin()
	if err != nil {
		log.Fatalf("failed to to open current window: %v\n", err)
	}
	defer w.CloseFiles()
	pos, fname, err := w.Position()
	if err != nil {
		log.Fatal(err)
	}
	s, err := startServerForFile(fname)
	if err != nil {
		log.Fatalf("cound not start language server: %v\n", err)
	}
	defer s.Close()

	b, err := w.ReadAll("body")
	if err != nil {
		log.Fatalf("failed to read source body: %v\n", err)
	}
	if err = s.lsp.DidOpen(fname, b); err != nil {
		log.Fatalf("DidOpen failed: %v\n", err)
	}
	defer func() {
		if err = s.lsp.DidClose(fname); err != nil {
			log.Printf("DidClose failed: %v\n", err)
		}
	}()

	switch flag.Arg(0) {
	case "comp":
		err = s.lsp.Completion(pos, os.Stdout)
	case "def":
		err = s.lsp.Definition(pos)
	case "fmt":
		err = s.lsp.Format(pos.TextDocument.URI, w)
	case "hov":
		err = s.lsp.Hover(pos, os.Stdout)
	case "refs":
		err = s.lsp.References(pos, os.Stdout)
	case "rn":
		if flag.NArg() < 2 {
			usage()
		}
		err = s.lsp.Rename(pos, flag.Arg(1))
	case "sig":
		err = s.lsp.SignatureHelp(pos, os.Stdout)
	case "syms":
		err = s.lsp.Symbols(pos.TextDocument.URI, os.Stdout)
	default:
		log.Printf("unknown command %q\n", flag.Arg(0))
		os.Exit(1)
	}
	if err != nil {
		log.Fatalf("%v\n", err)
	}
}

func formatWin(id int) error {
	w, err := openWin(id)
	if err != nil {
		return err
	}
	uri, fname, err := w.DocumentURI()
	if err != nil {
		return err
	}
	s, err := startServerForFile(fname)
	if err != nil {
		return nil // unknown language server
	}
	b, err := w.ReadAll("body")
	if err != nil {
		log.Fatalf("failed to read source body: %v\n", err)
	}
	if err := s.lsp.DidOpen(fname, b); err != nil {
		log.Fatalf("DidOpen failed: %v\n", err)
	}
	defer func() {
		if err := s.lsp.DidClose(fname); err != nil {
			log.Printf("DidClose failed: %v\n", err)
		}
	}()
	return s.lsp.Format(uri, w)
}

func monitor() {
	alog, err := acme.Log()
	if err != nil {
		panic(err)
	}
	defer alog.Close()
	for {
		ev, err := alog.Read()
		if err != nil {
			panic(err)
		}
		if ev.Op == "put" {
			if err = formatWin(ev.ID); err != nil {
				log.Printf("formating window %v failed: %v\n", ev.ID, err)
			}
		}
	}
}

type serverFlag []serverInfo

func (sf *serverFlag) String() string {
	return fmt.Sprintf("%v", []serverInfo(*sf))
}

func (sf *serverFlag) Set(val string) error {
	f := strings.SplitN(val, ":", 2)
	if len(f) != 2 {
		return errors.New("bad flag value")
	}
	re, err := regexp.Compile(f[0])
	if err != nil {
		return err
	}
	*sf = append(*sf, serverInfo{
		re:   re,
		args: strings.Fields(f[1]),
	})
	return nil
}
