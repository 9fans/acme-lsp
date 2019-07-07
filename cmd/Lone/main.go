package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/fhs/acme-lsp/internal/lsp/acmelsp"
	"github.com/fhs/acme-lsp/internal/lsp/client"
)

//go:generate ../../scripts/mkdocs.sh

const mainDoc = `The program Lone is a standalone client for the acme text editor that
interacts with a Language Server. (deprecated by L)

Note: this program is similar to the L command, except it also does
the work of acme-lsp command by executing a LSP server on demand. It's
recommended to use the L and acme-lsp commands instead, which takes
advantage of LSP server caches and should give faster responses.

A Language Server implements the Language Server Protocol
(see https://langserver.org/), which provides language features
like auto complete, go to definition, find all references, etc.
Lone depends on one or more language servers already being installed
in the system.  See this page of a list of language servers:
https://microsoft.github.io/language-server-protocol/implementors/servers/.

	Usage: Lone [flags] <sub-command> [args...]

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

	serverSet := client.NewServerSet(acmelsp.DefaultConfig())

	// golang.org/x/tools/cmd/gopls is not ready. It hasn't implmented
	// references, and rename yet.
	//serverSet.Register(`\.go$`, []string{"gopls"})
	serverSet.Register(`\.go$`, []string{"go-langserver", "-gocodecompletion"})
	serverSet.Register(`\.py$`, []string{"pyls"})
	//serverSet.Register(`\.c$`, []string{"cquery"})

	serverSet, _ = acmelsp.ParseFlags(serverSet)
	defer serverSet.CloseAll()

	if flag.NArg() < 1 {
		usage()
	}

	fm, err := acmelsp.NewFileManager(serverSet)
	if err != nil {
		log.Fatalf("failed to create file manager: %v\n", err)
	}
	switch flag.Arg(0) {
	case "win":
		if flag.NArg() < 2 {
			usage()
		}
		acmelsp.Watch(serverSet, fm, flag.Arg(1))
		return

	case "monitor":
		acmelsp.ManageFiles(serverSet, fm)
		return

	case "servers":
		serverSet.PrintTo(os.Stdout)
		return
	}

	cmd, err := acmelsp.CurrentWindowCmd(serverSet, fm)
	if err != nil {
		log.Fatalf("CurrentWindowCmd failed: %v\n", err)
	}
	defer cmd.Close()

	switch flag.Arg(0) {
	case "comp":
		err = cmd.Completion(false)
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
