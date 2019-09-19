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
	"strings"

	p9client "9fans.net/go/plan9/client"
	"github.com/fhs/acme-lsp/internal/golang_org_x_tools/jsonrpc2"
	"github.com/fhs/acme-lsp/internal/lsp"
	"github.com/fhs/acme-lsp/internal/lsp/acmelsp"
	"github.com/fhs/acme-lsp/internal/lsp/protocol"
	"github.com/fhs/acme-lsp/internal/lsp/proxy"
	"github.com/pkg/errors"
)

//go:generate ../../scripts/mkdocs.sh

const mainDoc = `The program L sends messages to the Language Server Protocol
proxy server acme-lsp.

L is usually run from within the acme text editor, where $winid
environment variable is set to the ID of currently focused window.
It sends this ID to acme-lsp, which uses it to compute the context for
LSP commands. Note: L merely asks acme-lsp to run an LSP command--any
output of the command is printed to stdout by acme-lsp, not L.

If L is run outside of acme (therefore $winid is not set), L will
attempt to find the focused window ID by connecting to acmefocused
(https://godoc.org/github.com/fhs/acme-lsp/cmd/acmefocused).

	Usage: L <sub-command> [args...]

List of sub-commands:

	comp [-e]
		Print candidate completions at current cursor position. If
		-e (edit) flag is given and there is only one candidate,
		the completion is applied instead of being printed.

	def
		Find where identifier at the cursor position is define and
		send the location to the plumber.

	fmt
		Format current window buffer.

	hov
		Show more information about the identifier under the cursor
		("hover").

	refs
		List locations where the identifier under the cursor is used
		("references").

	rn <newname>
		Rename the identifier under the cursor to newname.

	sig
		Show signature help for the function, method, etc. under
		the cursor.

	syms
		List symbols in the current file.

	type
		Find where the type of identifier at the cursor position is define
		and send the location to the plumber.

	assist [comp|hov|sig]
		A new window is created where completion (comp), hover
		(hov), or signature help (sig) output is shown depending
		on the cursor position in the focused window and the
		text surrounding the cursor. If the optional argument is
		given, the output will be limited to only that command.
		Note: this is a very experimental feature, and may not
		be very useful in practice.

	ws
		List current set of workspace directories.

	ws+ [directories...]
		Add given directories to the set of workspace directories.
		Current working directory is added if no directory is specified.

	ws- [directories...]
		Remove given directories to the set of workspace directories.
		Current working directory is removed if no directory is specified.
`

func usage() {
	os.Stderr.Write([]byte(mainDoc))
	fmt.Fprintf(os.Stderr, "\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	flag.Usage = usage
	flag.Parse()

	err := run(flag.Args())
	if err != nil {
		log.Fatalf("%v\n", err)
	}
}

func run(args []string) error {
	ctx := context.Background()

	if len(args) == 0 {
		usage()
	}

	conn, err := net.Dial("unix", acmelsp.ProxyAddr())
	if err != nil {
		return fmt.Errorf("dial failed: %v", err)
	}
	defer conn.Close()

	stream := jsonrpc2.NewHeaderStream(conn, conn)
	ctx, rpc, server := proxy.NewClient(ctx, stream, nil)
	go rpc.Run(ctx)

	switch args[0] {
	case "ws":
		folders, err := server.WorkspaceFolders(ctx)
		if err != nil {
			return err
		}
		for _, d := range folders {
			fmt.Printf("%v\n", d.Name)
		}
		return nil
	case "ws+":
		dirs, err := dirsOrCurrentDir(args[1:])
		if err != nil {
			return err
		}
		return server.DidChangeWorkspaceFolders(ctx, &protocol.DidChangeWorkspaceFoldersParams{
			Event: protocol.WorkspaceFoldersChangeEvent{
				Added: dirs,
			},
		})
	case "ws-":
		dirs, err := dirsOrCurrentDir(args[1:])
		if err != nil {
			return err
		}
		return server.DidChangeWorkspaceFolders(ctx, &protocol.DidChangeWorkspaceFoldersParams{
			Event: protocol.WorkspaceFoldersChangeEvent{
				Removed: dirs,
			},
		})
	}

	sendMsg := func(attr map[string]string, args ...string) error {
		return sendMessageWithWinID(ctx, server, attr, args...)
	}

	winid, err := getWinID()
	if err != nil {
		return err
	}
	rc := acmelsp.NewRemoteCmd(server, winid)

	switch args[0] {
	case "comp":
		args = args[1:]
		return rc.Completion(ctx, len(args) > 0 && args[0] == "-e")
	case "def":
		return rc.Definition(ctx)
	case "fmt":
		return sendMsg(nil, "format")
	case "hov":
		return rc.Hover(ctx)
	case "refs":
		return rc.References(ctx)
	case "rn":
		args = args[1:]
		if len(args) < 1 {
			usage()
		}
		return rc.Rename(ctx, args[0])
	case "sig":
		return rc.SignatureHelp(ctx)
	case "syms":
		return rc.DocumentSymbol(ctx)
	case "type":
		return rc.TypeDefinition(ctx)
	case "win", "assist": // "win" is deprecated
		args = args[1:]
		if len(args) == 0 {
			return sendMsg(nil, "watch-auto")
		}
		switch args[0] {
		case "comp":
			return sendMsg(nil, "watch-completion")
		case "sig":
			return sendMsg(nil, "watch-signature")
		case "hov":
			return sendMsg(nil, "watch-hover")
		case "auto":
			return sendMsg(nil, "watch-auto")
		}
		return fmt.Errorf("unknown assist command %q", args[0])
	}
	return fmt.Errorf("unknown command %q", args[0])
}

func sendMessageWithWinID(ctx context.Context, server proxy.Server, attr map[string]string, args ...string) error {
	winid, err := getFocusedWinID(filepath.Join(p9client.Namespace(), "acmefocused"))
	if err != nil {
		return errors.Wrap(err, "could not get focused window ID")
	}
	if attr == nil {
		attr = make(map[string]string)
	}
	attr["winid"] = winid
	return sendMessage(ctx, server, attr, args...)
}

func getWinID() (int, error) {
	winid, err := getFocusedWinID(filepath.Join(p9client.Namespace(), "acmefocused"))
	if err != nil {
		return 0, errors.Wrap(err, "could not get focused window ID")
	}
	n, err := strconv.Atoi(winid)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to parse $winid")
	}
	return n, nil
}

func sendMessage(ctx context.Context, server proxy.Server, attr map[string]string, args ...string) error {
	m := &proxy.Message{
		Data: strings.Join(args, " "),
		Attr: attr,
	}
	return server.SendMessage(ctx, m)
}

func dirsOrCurrentDir(dirs []string) ([]protocol.WorkspaceFolder, error) {
	if len(dirs) == 0 {
		d, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		dirs = []string{d}
	}
	return lsp.DirsToWorkspaceFolders(dirs)
}

func getFocusedWinID(addr string) (string, error) {
	winid := os.Getenv("winid")
	if winid == "" {
		conn, err := net.Dial("unix", addr)
		if err != nil {
			return "", errors.Wrap(err, "$winid is empty and could not dial acmefocused")
		}
		defer conn.Close()
		b, err := ioutil.ReadAll(conn)
		if err != nil {
			return "", errors.Wrap(err, "$winid is empty and could not read acmefocused")
		}
		return string(bytes.TrimSpace(b)), nil
	}
	return winid, nil
}
