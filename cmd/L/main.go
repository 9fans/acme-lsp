package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/fhs/acme-lsp/internal/lsp/acmelsp"
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
	defer serverSet.CloseAll()

	if flag.NArg() < 1 {
		usage()
	}
	switch flag.Arg(0) {
	case "win":
		if flag.NArg() < 2 {
			usage()
		}
		acmelsp.Watch(serverSet, flag.Arg(1))
		return

	case "monitor":
		acmelsp.FormatOnPut(serverSet)
		return

	case "servers":
		serverSet.PrintTo(os.Stdout)
		return
	}

	cmd, err := acmelsp.CurrentWindowCmd(serverSet)
	if err != nil {
		log.Fatalf("CurrentWindowCmd failed: %v\n", err)
	}
	defer cmd.Close()

	switch flag.Arg(0) {
	case "comp":
		err = cmd.Completion()
	case "def":
		err = cmd.Definition()
	case "fmt":
		err = cmd.Format()
	case "hov":
		err = cmd.Hover()
	case "refs":
		err = cmd.References()
	case "rn":
		if flag.NArg() < 2 {
			usage()
		}
		err = cmd.Rename(flag.Arg(1))
	case "sig":
		err = cmd.SignatureHelp()
	case "syms":
		err = cmd.Symbols()
	default:
		log.Printf("unknown command %q\n", flag.Arg(0))
		os.Exit(1)
	}
	if err != nil {
		log.Fatalf("%v\n", err)
	}
}
