package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"9fans.net/acme-lsp/internal/acmeutil"
	"9fans.net/acme-lsp/internal/lsp/acmelsp"
	"9fans.net/acme-lsp/internal/lsp/acmelsp/config"
	"9fans.net/acme-lsp/internal/lsp/cmd"
	p9client "github.com/fhs/9fans-go/plan9/client"
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
		Organize imports and format current window buffer.

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
	cfg := cmd.Setup(config.LangServerFlags)

	err := run(cfg, flag.Args())
	if err != nil {
		log.Fatalf("%v", err)
	}
}

func run(cfg *config.Config, args []string) error {
	serverSet, err := acmelsp.NewServerSet(cfg, acmelsp.NewDiagnosticsWriter())
	if err != nil {
		return fmt.Errorf("failed to create server set: %v", err)
	}
	defer serverSet.CloseAll()

	if len(serverSet.Data) == 0 {
		return fmt.Errorf("no servers found in the configuration file or command line flags")
	}

	if len(args) == 0 {
		usage()
	}

	fm, err := acmelsp.NewFileManager(serverSet, cfg)
	if err != nil {
		return fmt.Errorf("failed to create file manager: %v", err)
	}
	switch args[0] {
	case "win", "assist": // "win" is deprecated
		assist := "auto"
		if len(args) >= 2 {
			assist = args[1]
		}
		if err := acmelsp.Assist(serverSet, assist); err != nil {
			return fmt.Errorf("assist failed: %v", err)
		}
		return nil

	case "monitor":
		fm.Run()
		return nil

	case "servers":
		serverSet.PrintTo(os.Stdout)
		return nil
	}

	winid, err := getWinID()
	if err != nil {
		return err
	}
	name, err := acmeutil.Filename(winid)
	if err != nil {
		return err
	}

	rc, err := acmelsp.CurrentWindowRemoteCmd(serverSet, fm)
	if err != nil {
		return fmt.Errorf("CurrentWindowRemoteCmd failed: %v", err)
	}

	ctx := context.Background()

	switch args[0] {
	case "comp":
		err = rc.Completion(ctx, acmelsp.CompleteNoEdit)
	case "def":
		err = rc.Definition(ctx, false)
	case "fmt":
		return rc.OrganizeImportsAndFormat(ctx, serverSet.FormatOptionsForFile(name))
	case "hov":
		err = rc.Hover(ctx)
	case "refs":
		err = rc.References(ctx)
	case "rn":
		if len(args) < 2 {
			usage()
		}
		err = rc.Rename(ctx, args[1])
	case "sig":
		err = rc.SignatureHelp(ctx)
	case "syms":
		err = rc.DocumentSymbol(ctx)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
	return err
}

func getWinID() (int, error) {
	winid, err := getFocusedWinID(filepath.Join(p9client.Namespace(), "acmefocused"))
	if err != nil {
		return 0, fmt.Errorf("could not get focused window ID: %v", err)
	}
	n, err := strconv.Atoi(winid)
	if err != nil {
		return 0, fmt.Errorf("failed to parse $winid: %v", err)
	}
	return n, nil
}

func getFocusedWinID(addr string) (string, error) {
	winid := os.Getenv("winid")
	if winid == "" {
		conn, err := net.Dial("unix", addr)
		if err != nil {
			return "", fmt.Errorf("$winid is empty and could not dial acmefocused: %v", err)
		}
		defer conn.Close()
		b, err := ioutil.ReadAll(conn)
		if err != nil {
			return "", fmt.Errorf("$winid is empty and could not read acmefocused: %v", err)
		}
		return string(bytes.TrimSpace(b)), nil
	}
	return winid, nil
}
