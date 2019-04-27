package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"9fans.net/go/plan9"
	"9fans.net/go/plumb"
	"github.com/pkg/errors"
)

//go:generate ../../scripts/mkdocs.sh

const mainDoc = `The program L sends messages to the Language Server Protocol
proxy server acme-lsp.

L is usually run from within the acme text editor, where $winid
environment variable is set to the window ID.  It sends $winid to
acme-lsp, which uses it to compute the context for LSP commands.

	Usage: L <sub-command> [args...]

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
	flag.Parse()

	// This is racy but better than nothing.
	if !portOpen() {
		log.Fatalf("lsp plumber port is not open. Make sure it's open and acme-lsp is running.\n")
	}

	err := run(flag.Args())
	if err != nil {
		if strings.Contains(err.Error(), "no start action for plumb message") {
			// Clarify confusing error.
			err = fmt.Errorf("acme-lsp is not running")
		}
		log.Fatalf("%v\n", err)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
	}
	switch args[0] {
	case "comp":
		return plumbCmd(nil, "completion")
	case "def":
		return plumbCmd(nil, "definition")
	case "fmt":
		return plumbCmd(nil, "format")
	case "hov":
		return plumbCmd(nil, "hover")
	case "refs":
		return plumbCmd(nil, "references")
	case "rn":
		if len(args) < 2 {
			usage()
		}
		attr := &plumb.Attribute{
			Name:  "newname",
			Value: args[1],
		}
		return plumbCmd(attr, "rename")
	case "sig":
		return plumbCmd(nil, "signature")
	case "syms":
		return plumbCmd(nil, "symbols")
	case "win":
		if len(args) < 2 {
			usage()
		}
		switch args[1] {
		case "comp":
			return plumbCmd(nil, "watch-completion")
		case "sig":
			return plumbCmd(nil, "watch-signature")
		case "hov":
			return plumbCmd(nil, "watch-hover")
		}
		return fmt.Errorf("unknown win command %q", flag.Arg(1))
	}
	return fmt.Errorf("unknown command %q", args[0])
}

func plumbCmd(attr *plumb.Attribute, args ...string) error {
	winid := os.Getenv("winid")
	if winid == "" {
		return fmt.Errorf("$winid is empty")
	}

	p, err := plumb.Open("send", plan9.OWRITE)
	if err != nil {
		return errors.Wrap(err, "failed to open plumber")
	}
	defer p.Close()

	attr = &plumb.Attribute{
		Name:  "winid",
		Value: winid,
		Next:  attr,
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	m := &plumb.Message{
		Src:  "L",
		Dst:  "lsp",
		Dir:  cwd,
		Type: "text",
		Attr: attr,
		Data: []byte(strings.Join(args, " ")),
	}
	return m.Send(p)
}

func portOpen() bool {
	fid, err := plumb.Open("lsp", plan9.OREAD|plan9.OCEXEC)
	defer fid.Close()
	return err == nil
}
