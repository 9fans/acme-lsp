package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"9fans.net/go/plan9"
	p9client "9fans.net/go/plan9/client"
	"9fans.net/go/plumb"
	"github.com/fhs/acme-lsp/internal/lsp/acmelsp"
	"github.com/fhs/acme-lsp/internal/lsp/client"
	"github.com/pkg/errors"
)

//go:generate ../../scripts/mkdocs.sh

const mainDoc = `The program acme-lsp is a client for the acme text editor that
acts as a proxy for a set of Language Server Protocol servers.

A Language Server implements the Language Server Protocol
(see https://langserver.org/), which provides language features
like auto complete, go to definition, find all references, etc.
Acme-lsp depends on one or more language servers already being
installed in the system.  See this page of a list of language servers:
https://microsoft.github.io/language-server-protocol/implementors/servers/.

Acme-lsp executes or connects to a set of LSP servers specified using the
-server or -dial flags.  It then listens for messages sent to the plumber
(https://9fans.github.io/plan9port/man/man4/plumber.html) port named
"lsp".  The messages direct acme-lsp to run commands on the LSP servers
and apply/show the results in acme.  The format of the plumbing messages
is implementation dependent and subject to change. The L command should
be used to send the messages instead of plumb(1) command.

This following plumbing rule must be added to $HOME/lib/plumbing:

	# declarations of ports without rules
	plumb to lsp

and then the rules must be reload by running:

	cat $HOME/lib/plumbing | 9p write plumb/rules

Acme-lsp also watches for Put executed in an acme window, organizes
import paths in the window and formats it.

	Usage: acme-lsp [flags]
`

func usage() {
	os.Stderr.Write([]byte(mainDoc))
	fmt.Fprintf(os.Stderr, "\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	flag.Usage = usage
	ss, _ := acmelsp.ParseFlags(nil)

	if len(ss.Data) == 0 {
		log.Fatalf("No servers specified. Specify either -server or -dial flag. Run with -help for usage help.\n")
	}
	go acmelsp.FormatOnPut(ss)
	readPlumb(ss)
}

func readPlumb(ss *client.ServerSet) {
	for {
		var fid *p9client.Fid

		retrying := false
		for {
			var err error
			fid, err = plumb.Open("lsp", plan9.OREAD|plan9.OCEXEC)
			if err == nil {
				break
			}
			if !retrying {
				log.Printf("plumb open failed: %v", err)
				fmt.Printf("Make sure plumber is running with this empty rule:\n")
				fmt.Printf("\tplumb to lsp\n")
				retrying = true
			}
			// wait and retry
			time.Sleep(2 * time.Second)
		}

		rd := bufio.NewReader(fid)

		for {
			var m plumb.Message
			err := m.Recv(rd)
			if err != nil {
				log.Printf("plumb recv failed: %v", err)
				break
			}
			attr := make(map[string]string)
			for a := m.Attr; a != nil; a = a.Next {
				attr[a.Name] = a.Value
			}
			err = run(ss, string(m.Data), attr)
			if err != nil {
				log.Printf("%v failed: %v\n", string(m.Data), err)
			}
		}

		fid.Close()
	}
}

func run(ss *client.ServerSet, data string, attr map[string]string) error {
	args := strings.Fields(data)
	switch args[0] {
	case "workspaces":
		fmt.Printf("Workspaces:\n")
		for _, d := range ss.Workspaces() {
			fmt.Printf(" %v\n", d)
		}
		return nil
	case "workspaces-add":
		return ss.AddWorkspaces(args[1:])
	case "workspaces-remove":
		return ss.RemoveWorkspaces(args[1:])
	}

	winid, err := strconv.Atoi(attr["winid"])
	if err != nil {
		return errors.Wrap(err, "failed to parse $winid")
	}
	cmd, err := acmelsp.WindowCmd(ss, winid)
	if err != nil {
		return err
	}

	switch args[0] {
	case "completion":
		return cmd.Completion()
	case "definition":
		return cmd.Definition()
	case "type-definition":
		return cmd.TypeDefinition()
	case "format":
		return cmd.Format()
	case "hover":
		return cmd.Hover()
	case "implementation":
		return cmd.Implementation()
	case "references":
		return cmd.References()
	case "rename":
		return cmd.Rename(attr["newname"])
	case "signature":
		return cmd.SignatureHelp()
	case "symbols":
		return cmd.Symbols()
	case "watch-completion":
		go acmelsp.Watch(ss, "comp")
		return nil
	case "watch-signature":
		go acmelsp.Watch(ss, "sig")
		return nil
	case "watch-hover":
		go acmelsp.Watch(ss, "hov")
		return nil
	}
	return fmt.Errorf("unknown command %v", args[0])
}
