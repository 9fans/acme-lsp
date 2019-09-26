package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/fhs/acme-lsp/internal/acme"
	"github.com/fhs/acme-lsp/internal/lsp/acmelsp"
	"github.com/fhs/acme-lsp/internal/lsp/acmelsp/config"
)

//go:generate ../../scripts/mkdocs.sh

const mainDoc = `The program Lone is a standalone client for the acme text editor that
interacts with a Language Server.

Deprecated: This program is similar to the L command, except it also does
the work of acme-lsp command by executing a LSP server on-demand. It's
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

	assist [comp|hov|sig]
		A new window is created where completion (comp), hover
		(hov), or signature help (sig) output is shown depending
		on the cursor position in the focused window and the
		text surrounding the cursor. If the optional argument is
		given, the output will be limited to only that command.
		Note: this is a very experimental feature, and may not
		be very useful in practice.
`

func usage() {
	os.Stderr.Write([]byte(mainDoc))
	fmt.Fprintf(os.Stderr, "\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	flag.Usage = usage

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config file: %v", err)
	}
	cfgServers := []config.LegacyLanguageServer{
		// Use go-langserver insead of gopls for backward compatibility.
		{
			Pattern: `\.go$`,
			Command: []string{"go-langserver", "-gocodecompletion"},
		},
		{
			Pattern: `\.py$`,
			Command: []string{"pyls"},
		},
		//{
		//	Pattern: `\.c$`,
		//	Command: []string{"cquery"},
		//},
	}
	cfg.LegacyLanguageServers = append(cfg.LegacyLanguageServers, cfgServers...)

	err = config.ParseFlags(cfg, config.LangServerFlags, flag.CommandLine, os.Args[1:])
	if err != nil {
		// Unreached since flag.CommandLine uses flag.ExitOnError.
		log.Fatalf("failed to parse flags: %v\n", err)
	}

	// Setup custom acme package
	acme.Network = cfg.AcmeNetwork
	acme.Address = cfg.AcmeAddress

	serverSet, err := acmelsp.NewServerSet(cfg)
	if err != nil {
		log.Fatalf("failed to create server set: %v\n", err)
	}
	defer serverSet.CloseAll()

	if flag.NArg() < 1 {
		usage()
	}

	fm, err := acmelsp.NewFileManager(serverSet, cfg)
	if err != nil {
		log.Fatalf("failed to create file manager: %v\n", err)
	}
	switch flag.Arg(0) {
	case "win", "assist": // "win" is deprecated
		assist := "auto"
		if flag.NArg() >= 2 {
			assist = flag.Arg(1)
		}
		if err := acmelsp.Assist(serverSet, assist); err != nil {
			log.Fatalf("assist failed: %v", err)
		}
		return

	case "monitor":
		fm.Run()
		return

	case "servers":
		serverSet.PrintTo(os.Stdout)
		return
	}

	rc, err := acmelsp.CurrentWindowRemoteCmd(serverSet, fm)
	if err != nil {
		log.Fatalf("CurrentWindowRemoteCmd failed: %v\n", err)
	}

	ctx := context.Background()

	switch flag.Arg(0) {
	case "comp":
		err = rc.Completion(ctx, false)
	case "def":
		err = rc.Definition(ctx)
	case "fmt":
		err = rc.OrganizeImportsAndFormat(ctx)
	case "hov":
		err = rc.Hover(ctx)
	case "refs":
		err = rc.References(ctx)
	case "rn":
		if flag.NArg() < 2 {
			usage()
		}
		err = rc.Rename(ctx, flag.Arg(1))
	case "sig":
		err = rc.SignatureHelp(ctx)
	case "syms":
		err = rc.DocumentSymbol(ctx)
	default:
		log.Printf("unknown command %q\n", flag.Arg(0))
		os.Exit(1)
	}
	if err != nil {
		log.Fatalf("%v\n", err)
	}
}
