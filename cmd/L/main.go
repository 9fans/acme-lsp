package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"9fans.net/go/plan9"
	"9fans.net/go/plumb"
	"github.com/fhs/acme-lsp/internal/lsp/client"
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

	type
		Find where the type of identifier at the cursor position is define
		and send the location to the plumber.

	win <command>
		The command argument can be either "comp", "hov" or "sig". A
		new window is created where the output of the given command
		is shown each time cursor position is changed.

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
		return plumbAcmeCmd(nil, "completion")
	case "def":
		return plumbAcmeCmd(nil, "definition")
	case "fmt":
		return plumbAcmeCmd(nil, "format")
	case "hov":
		return plumbAcmeCmd(nil, "hover")
	case "refs":
		return plumbAcmeCmd(nil, "references")
	case "rn":
		if len(args) < 2 {
			usage()
		}
		attr := &plumb.Attribute{
			Name:  "newname",
			Value: args[1],
		}
		return plumbAcmeCmd(attr, "rename")
	case "sig":
		return plumbAcmeCmd(nil, "signature")
	case "syms":
		return plumbAcmeCmd(nil, "symbols")
	case "type":
		return plumbAcmeCmd(nil, "type-definition")
	case "win":
		if len(args) < 2 {
			usage()
		}
		switch args[1] {
		case "comp":
			return plumbAcmeCmd(nil, "watch-completion")
		case "sig":
			return plumbAcmeCmd(nil, "watch-signature")
		case "hov":
			return plumbAcmeCmd(nil, "watch-hover")
		case "auto":
			return plumbAcmeCmd(nil, "watch-auto")
		}
		return fmt.Errorf("unknown win command %q", flag.Arg(1))
	case "ws":
		return plumbCmd(nil, "workspaces")
	case "ws+":
		dirs, err := dirsOrCurrentDir(args[1:])
		if err != nil {
			return err
		}
		args = append([]string{"workspaces-add"}, dirs...)
		return plumbCmd(nil, args...)
	case "ws-":
		dirs, err := dirsOrCurrentDir(args[1:])
		if err != nil {
			return err
		}
		args = append([]string{"workspaces-remove"}, dirs...)
		return plumbCmd(nil, args...)
	}
	return fmt.Errorf("unknown command %q", args[0])
}

func plumbCmd(attr *plumb.Attribute, args ...string) error {
	p, err := plumb.Open("send", plan9.OWRITE)
	if err != nil {
		return errors.Wrap(err, "failed to open plumber")
	}
	defer p.Close()

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

func plumbAcmeCmd(attr *plumb.Attribute, args ...string) error {
	winid := os.Getenv("winid")
	if winid == "" {
		return fmt.Errorf("$winid is empty")
	}
	attr = &plumb.Attribute{
		Name:  "winid",
		Value: winid,
		Next:  attr,
	}
	return plumbCmd(attr, args...)
}

func portOpen() bool {
	fid, err := plumb.Open("lsp", plan9.OREAD|plan9.OCEXEC)
	if err != nil {
		return false
	}
	defer fid.Close()
	return true
}

func dirsOrCurrentDir(dirs []string) ([]string, error) {
	if len(dirs) == 0 {
		d, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		dirs = []string{d}
	}
	return client.AbsDirs(dirs)
}
