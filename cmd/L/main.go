package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"9fans.net/go/acme"
	"github.com/pkg/errors"
)

var debug = flag.Bool("debug", false, "turn on debugging prints")

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
	flag.Parse()

	if len(os.Args) < 2 {
		usage()
	}
	if os.Args[1] == "win" {
		if len(os.Args) < 3 {
			usage()
		}
		startServers()
		defer killServers()
		watch(os.Args[2])
		return
	}
	if os.Args[1] == "monitor" {
		startServers()
		defer killServers()
		monitor()
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

	switch os.Args[1] {
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
		if len(os.Args) < 3 {
			usage()
		}
		err = s.lsp.Rename(pos, os.Args[2])
	case "sig":
		err = s.lsp.SignatureHelp(pos, os.Stdout)
	default:
		log.Printf("unknown command %q\n", os.Args[1])
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
		return errors.Wrapf(err, "formatting window %v failed", id)
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
