package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
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

func readCurrentWinBody() ([]byte, error) {
	id, err := strconv.Atoi(os.Getenv("winid"))
	if err != nil {
		return nil, err
	}
	w, err := openWin(id)
	if err != nil {
		return nil, err
	}
	defer w.CloseFiles()

	return w.ReadAll("body")
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if len(os.Args) < 2 {
		usage()
	}
	if os.Args[1] == "watch" {
		if len(os.Args) < 3 {
			usage()
		}
		startServers()
		defer killServers()
		watch(os.Args[2])
		return
	}
	pos, _, err := getAcmePos()
	if err != nil {
		log.Fatal(err)
	}
	lang := lspLang(string(pos.TextDocument.URI))
	cmd, ok := serverCommands[lang]
	if !ok {
		log.Fatalf("unknown language %q\n", lang)
	}
	s, err := startServer(lang, cmd)
	if err != nil {
		log.Fatalf("cound not start %v server: %v\n", lang, err)
	}
	defer s.Kill()

	b, err := readCurrentWinBody()
	if err != nil {
		log.Fatalf("failed to read source body: %v\n", err)
	}
	fname := uriToFilename(string(pos.TextDocument.URI))
	err = s.lsp.DidOpen(fname, b)
	if err != nil {
		log.Fatalf("DidOpen failed: %v\n", err)
	}
	defer func() {
		err = s.lsp.DidClose(fname)
		if err != nil {
			log.Printf("DidClose failed: %v\n", err)
		}
	}()

	switch os.Args[1] {
	case "comp":
		err = s.lsp.Completion(pos, os.Stdout)
	case "def":
		err = s.lsp.Definition(pos)
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