package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/fhs/acme-lsp/internal/acmeutil"
	"github.com/fhs/acme-lsp/internal/lsp/acmelsp"
	"github.com/fhs/acme-lsp/internal/lsp/text"
)

//go:generate ./mkdocs.sh

const mainDoc = `The program L is a client for the acme text editor that interacts with a
Language Server.

A Language Server implements the Language Server Protocol
(see https://langserver.org/), which provides language features
like auto complete, go to definition, find all references, etc.
L depends on one or more language servers already being installed
in the system.  See this page of a list of language servers:
https://microsoft.github.io/language-server-protocol/implementors/servers/.

	Usage: L [flags] <sub-command> [args...]

List of sub-commands:

	comp
		Show auto-completion for the current cursor position.

	def
		Find where identifier at the cursor position is define and
		send the location to the plumber.

	fmt
		Format current window buffer.

	hov
		Show more information about the identifier under the cursor
		("hover").

	monitor
		Format window buffer after each Put.

	refs
		List locations where the identifier under the cursor is used
		("references").

	rn <newname>
		Rename the identifier under the cursor to newname.

	servers
		Print list of known language servers.

	sig
		Show signature help for the function, method, etc. under
		the cursor.

	syms
		List symbols in the current file.

	win <command>
		The command argument can be either "comp", "hov" or "sig". A
		new window is created where the output of the given command
		is shown each time cursor position is changed.
`

func usage() {
	os.Stderr.Write([]byte(mainDoc))
	fmt.Fprintf(os.Stderr, "\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	flag.Usage = usage
	serverSet, _ := acmelsp.ParseFlags()

	if flag.NArg() < 1 {
		usage()
	}
	switch flag.Arg(0) {
	case "win":
		if flag.NArg() < 2 {
			usage()
		}
		acmelsp.Watch(serverSet, flag.Arg(1))
		serverSet.CloseAll()
		return

	case "monitor":
		acmelsp.FormatOnPut(serverSet)
		serverSet.CloseAll()
		return

	case "servers":
		serverSet.PrintTo(os.Stdout)
		return
	}
	w, err := acmeutil.OpenCurrentWin()
	if err != nil {
		log.Fatalf("failed to to open current window: %v\n", err)
	}
	defer w.CloseFiles()
	pos, fname, err := text.Position(w)
	if err != nil {
		log.Fatal(err)
	}
	s, err := serverSet.StartForFile(fname)
	if err != nil {
		log.Fatalf("cound not start language server: %v\n", err)
	}
	defer s.Close()

	b, err := w.ReadAll("body")
	if err != nil {
		log.Fatalf("failed to read source body: %v\n", err)
	}
	if err = s.Conn.DidOpen(fname, b); err != nil {
		log.Fatalf("DidOpen failed: %v\n", err)
	}
	defer func() {
		if err = s.Conn.DidClose(fname); err != nil {
			log.Printf("DidClose failed: %v\n", err)
		}
	}()

	switch flag.Arg(0) {
	case "comp":
		err = s.Conn.Completion(pos, os.Stdout)
	case "def":
		err = acmelsp.PlumbDefinition(s.Conn, pos)
	case "fmt":
		err = acmelsp.FormatFile(s.Conn, pos.TextDocument.URI, w)
	case "hov":
		err = s.Conn.Hover(pos, os.Stdout)
	case "refs":
		err = s.Conn.References(pos, os.Stdout)
	case "rn":
		if flag.NArg() < 2 {
			usage()
		}
		err = acmelsp.Rename(s.Conn, pos, flag.Arg(1))
	case "sig":
		err = s.Conn.SignatureHelp(pos, os.Stdout)
	case "syms":
		err = s.Conn.Symbols(pos.TextDocument.URI, os.Stdout)
	default:
		log.Printf("unknown command %q\n", flag.Arg(0))
		os.Exit(1)
	}
	if err != nil {
		log.Fatalf("%v\n", err)
	}
}
