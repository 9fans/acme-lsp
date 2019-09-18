// Package acmelsp defines helper functions for implementation of acme-lsp commands.
package acmelsp

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"9fans.net/go/acme"
	"9fans.net/go/plan9"
	"9fans.net/go/plumb"
	"github.com/fhs/acme-lsp/internal/acmeutil"
	"github.com/fhs/acme-lsp/internal/lsp"
	"github.com/fhs/acme-lsp/internal/lsp/protocol"
	"github.com/fhs/acme-lsp/internal/lsp/text"
	"github.com/pkg/errors"
)

// Cmd contains the states required to execute an LSP command in an acme window.
type Cmd struct {
	Client   *lsp.Client
	win      *acmeutil.Win
	pos      *protocol.TextDocumentPositionParams
	filename string
}

func CurrentWindowCmd(ss *lsp.ServerSet, fm *FileManager) (*Cmd, error) {
	id, err := strconv.Atoi(os.Getenv("winid"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse $winid")
	}
	return WindowCmd(ss, fm, id)
}

func WindowCmd(ss *lsp.ServerSet, fm *FileManager, winid int) (*Cmd, error) {
	w, err := acmeutil.OpenWin(winid)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to to open window %v", winid)
	}
	pos, fname, err := text.Position(w)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get text position")
	}
	srv, found, err := ss.StartForFile(fname)
	if err != nil {
		return nil, errors.Wrap(err, "cound not start language server")
	}
	if !found {
		return nil, fmt.Errorf("no language server for filename %q", fname)
	}

	// In case the window has unsaved changes (it's dirty),
	// send changes to LSP server.
	if err = fm.didChange(winid, fname); err != nil {
		return nil, errors.Wrap(err, "DidChange failed")
	}

	return &Cmd{
		Client:   srv.Client,
		win:      w,
		pos:      pos,
		filename: fname,
	}, nil
}

func (c *Cmd) Close() {
	c.win.CloseFiles()
}

func (c *Cmd) Completion(edit bool) error {
	items, err := c.Client.Completion1(c.pos)
	if err != nil {
		return err
	}
	if edit && len(items) == 1 {
		textEdit := items[0].TextEdit
		if textEdit == nil {
			// TODO(fhs): Use insertText or label instead.
			return fmt.Errorf("nil TextEdit in completion item")
		}
		if err := text.Edit(c.win, []protocol.TextEdit{*textEdit}); err != nil {
			return errors.Wrapf(err, "failed to apply completion edit")
		}
		return nil
	}
	printCompletionItems(os.Stdout, items)
	return nil
}

func printCompletionItems(w io.Writer, items []protocol.CompletionItem) {
	if len(items) == 0 {
		fmt.Fprintf(w, "no completion\n")
	}
	for _, item := range items {
		fmt.Fprintf(w, "%v %v\n", item.Label, item.Detail)
	}
}

func (c *Cmd) Definition() error {
	locations, err := c.Client.Definition(context.Background(), &protocol.DefinitionParams{
		TextDocumentPositionParams: *c.pos,
	})
	if err != nil {
		return err
	}
	return PlumbLocations(locations)
}

func (c *Cmd) TypeDefinition() error {
	locations, err := c.Client.TypeDefinition(c.pos)
	if err != nil {
		return err
	}
	return PlumbLocations(locations)
}

func (c *Cmd) Implementation() error {
	locations, err := c.Client.Implementation(c.pos)
	if err != nil {
		return err
	}
	return PlumbLocations(locations)
}

func (c *Cmd) Format() error {
	return FormatFile(c.Client, c.pos.TextDocument.URI, c.win)
}

func (c *Cmd) Hover() error {
	return c.Client.Hover1(c.pos, os.Stdout)
}

func (c *Cmd) References() error {
	return c.Client.References1(c.pos, os.Stdout)
}

// Rename renames the identifier at cursor position to newname.
func (c *Cmd) Rename(newname string) error {
	we, err := c.Client.Rename(context.Background(), &protocol.RenameParams{
		TextDocument: c.pos.TextDocument,
		Position:     c.pos.Position,
		NewName:      newname,
	})
	if err != nil {
		return err
	}
	return editWorkspace(we)
}

func (c *Cmd) SignatureHelp() error {
	return c.Client.SignatureHelp(c.pos, os.Stdout)
}

func (c *Cmd) Symbols() error {
	return c.Client.Symbols(c.pos.TextDocument.URI, os.Stdout)
}

// PlumbLocations sends the locations to the plumber.
func PlumbLocations(locations []protocol.Location) error {
	p, err := plumb.Open("send", plan9.OWRITE)
	if err != nil {
		return errors.Wrap(err, "failed to open plumber")
	}
	defer p.Close()
	for _, loc := range locations {
		err := plumbLocation(&loc).Send(p)
		if err != nil {
			return errors.Wrap(err, "failed to plumb location")
		}
	}
	return nil
}

func plumbLocation(loc *protocol.Location) *plumb.Message {
	// LSP uses zero-based offsets.
	// Place the cursor *before* the location range.
	pos := loc.Range.Start
	attr := &plumb.Attribute{
		Name:  "addr",
		Value: fmt.Sprintf("%v-#0+#%v", pos.Line+1, pos.Character),
	}
	return &plumb.Message{
		Src:  "acme-lsp",
		Dst:  "edit",
		Dir:  "/",
		Type: "text",
		Attr: attr,
		Data: []byte(text.ToPath(loc.URI)),
	}
}

// FormatFile organizes import paths and then formats the file f.
func FormatFile(c *lsp.Client, uri protocol.DocumentURI, f text.File) error {
	if c.ProvidesCodeAction(protocol.SourceOrganizeImports) {
		actions, err := c.OrganizeImports(uri)
		if err != nil {
			return err
		}
		for _, a := range actions {
			err := editWorkspace(a.Edit)
			if err != nil {
				return err
			}
		}
		if len(actions) > 0 {
			// Our file may or may not be among the workspace edits for import fixes.
			// We assume it is among the edits.
			// TODO(fhs): Skip if our file didn't have import changes.
			rd, err := f.Reader()
			if err != nil {
				return err
			}
			b, err := ioutil.ReadAll(rd)
			if err != nil {
				return err
			}
			err = c.DidChange(text.ToPath(uri), b)
			if err != nil {
				return err
			}
		}
	}
	edits, err := c.Format(uri)
	if err != nil {
		return err
	}
	if err := text.Edit(f, edits); err != nil {
		return errors.Wrapf(err, "failed to apply edits")
	}
	return nil
}

func editWorkspace(we *protocol.WorkspaceEdit) error {
	wins, err := acme.Windows()
	if err != nil {
		return errors.Wrapf(err, "failed to read list of acme index")
	}
	winid := make(map[string]int, len(wins))
	for _, info := range wins {
		winid[info.Name] = info.ID
	}

	for uri := range *we.Changes {
		fname := text.ToPath(uri)
		if _, ok := winid[fname]; !ok {
			return fmt.Errorf("%v: not open in acme", fname)
		}
	}
	for uri, edits := range *we.Changes {
		fname := text.ToPath(uri)
		id := winid[fname]
		w, err := acmeutil.OpenWin(id)
		if err != nil {
			return errors.Wrapf(err, "failed to open window %v", id)
		}
		if err := text.Edit(w, edits); err != nil {
			return errors.Wrapf(err, "failed to apply edits to window %v", id)
		}
		w.CloseFiles()
	}
	return nil
}

// ParseFlags adds some standard flags, parses all flags, and returns the server set and debug.
func ParseFlags(serverSet *lsp.ServerSet) (*lsp.ServerSet, bool) {
	serverSet, debug, err := ParseFlagSet(flag.CommandLine, os.Args[1:], serverSet)
	if err != nil {
		// Unreached since flag.CommandLine uses flag.ExitOnError.
		panic(err)
	}
	return serverSet, debug
}

func ParseFlagSet(f *flag.FlagSet, arguments []string, serverSet *lsp.ServerSet) (*lsp.ServerSet, bool, error) {
	var (
		userServers serverFlag
		dialServers serverFlag
	)

	debug := f.Bool("debug", false, "turn on debugging prints")
	workspaces := f.String("workspaces", "", "colon-separated list of initial workspace directories")
	f.Var(&userServers, "server", `language server command for filename match (e.g. '\.go$:gopls')`)
	f.Var(&dialServers, "dial", `language server address for filename match (e.g. '\.go$:localhost:4389')`)
	if err := f.Parse(arguments); err != nil {
		return nil, false, err
	}

	if *debug {
		lsp.Debug = true
	}

	if serverSet == nil {
		serverSet = lsp.NewServerSet(DefaultConfig())
	}
	if len(*workspaces) > 0 {
		folders, err := lsp.DirsToWorkspaceFolders(strings.Split(*workspaces, ":"))
		if err != nil {
			return nil, *debug, err
		}
		serverSet.InitWorkspaces(folders)
	}
	for _, sa := range userServers {
		serverSet.Register(sa.pattern, strings.Fields(sa.args))
	}
	for _, sa := range dialServers {
		serverSet.RegisterDial(sa.pattern, sa.args)
	}
	return serverSet, *debug, nil
}

type serverArgs struct {
	pattern, args string
}

type serverFlag []serverArgs

func (sf *serverFlag) String() string {
	return fmt.Sprintf("%v", []serverArgs(*sf))
}

func (sf *serverFlag) Set(val string) error {
	f := strings.SplitN(val, ":", 2)
	if len(f) != 2 {
		return errors.New("flag value must contain a colon")
	}
	*sf = append(*sf, serverArgs{
		pattern: f[0],
		args:    f[1],
	})
	return nil
}

func DefaultConfig() *lsp.Config {
	return &lsp.Config{
		DiagWriter: NewDiagnosticsWriter(),
		RootDir:    "/",
		Workspaces: nil,
	}
}
