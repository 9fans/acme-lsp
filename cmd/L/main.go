package main

import (
	"fmt"
	"log"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("usage: %v <command>\n", os.Args[0])
		os.Exit(2)
	}

	if os.Args[1] == "watch" {
		startServers()
		defer killServers()
		watch()
		return
	}
	pos, _, err := getAcmePos()
	if err != nil {
		log.Fatal(err)
	}
	lang := lspLang(string(pos.TextDocument.URI))
	s, err := startServer(lang, serverCommands[lang])
	if err != nil {
		log.Printf("cound not start %v server: %v\n", lang, err)
		return
	}
	defer s.Kill()

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
			log.Fatalf("new name argument missing")
		}
		err = s.lsp.Rename(pos, os.Args[2])
	default:
		log.Printf("unknown command %q\n", os.Args[1])
		os.Exit(1)
	}
	if err != nil {
		log.Fatalf("%v\n", err)
	}
}
