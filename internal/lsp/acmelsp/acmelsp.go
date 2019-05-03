// Package acmelsp defines helper functions for implementation of acme-lsp commands.
package acmelsp

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"

	"9fans.net/go/acme"
	"9fans.net/go/plan9"
	"9fans.net/go/plumb"
	"github.com/fhs/acme-lsp/internal/acmeutil"
	"github.com/fhs/acme-lsp/internal/lsp"
	"github.com/fhs/acme-lsp/internal/lsp/client"
	"github.com/fhs/acme-lsp/internal/lsp/text"
	"github.com/pkg/errors"
)

// Cmd contains the states required to execute an LSP command in an acme window.
type Cmd struct {
	conn     *client.Conn
	win      *acmeutil.Win
	pos      *lsp.TextDocumentPositionParams
	filename string
}

func CurrentWindowCmd(ss *client.ServerSet) (*Cmd, error) {
	id, err := strconv.Atoi(os.Getenv("winid"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse $winid")
	}
	return WindowCmd(ss, id)
}

func WindowCmd(ss *client.ServerSet, winid int) (*Cmd, error) {
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

	b, err := w.ReadAll("body")
	if err != nil {
		return nil, errors.Wrap(err, "failed to read source body")
	}
	if err = srv.Conn.DidOpen(fname, b); err != nil {
		return nil, errors.Wrap(err, "DidOpen failed")
	}

	return &Cmd{
		conn:     srv.Conn,
		win:      w,
		pos:      pos,
		filename: fname,
	}, nil
}

func (c *Cmd) Close() error {
	c.win.CloseFiles()
	return c.conn.DidClose(c.filename)
}

func (c *Cmd) Completion() error {
	return c.conn.Completion(c.pos, os.Stdout)
}

func (c *Cmd) Definition() error {
	locations, err := c.conn.Definition(c.pos)
	if err != nil {
		return err
	}
	return PlumbLocations(locations)
}

func (c *Cmd) TypeDefinition() error {
	locations, err := c.conn.TypeDefinition(c.pos)
	if err != nil {
		return err
	}
	return PlumbLocations(locations)
}

func (c *Cmd) Format() error {
	return FormatFile(c.conn, c.pos.TextDocument.URI, c.win)
}

func (c *Cmd) Hover() error {
	return c.conn.Hover(c.pos, os.Stdout)
}

func (c *Cmd) References() error {
	return c.conn.References(c.pos, os.Stdout)
}

func (c *Cmd) Rename(newname string) error {
	return Rename(c.conn, c.pos, newname)
}

func (c *Cmd) SignatureHelp() error {
	return c.conn.SignatureHelp(c.pos, os.Stdout)
}

func (c *Cmd) Symbols() error {
	return c.conn.Symbols(c.pos.TextDocument.URI, os.Stdout)
}

// PlumbLocations sends the locations to the plumber.
func PlumbLocations(locations []lsp.Location) error {
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

func plumbLocation(loc *lsp.Location) *plumb.Message {
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

func formatWin(serverSet *client.ServerSet, id int) error {
	w, err := acmeutil.OpenWin(id)
	if err != nil {
		return err
	}
	uri, fname, err := text.DocumentURI(w)
	if err != nil {
		return err
	}
	s, found, err := serverSet.StartForFile(fname)
	if err != nil {
		return err
	}
	if !found {
		return nil // unknown language server
	}
	b, err := w.ReadAll("body")
	if err != nil {
		log.Fatalf("failed to read source body: %v\n", err)
	}
	if err := s.Conn.DidOpen(fname, b); err != nil {
		log.Fatalf("DidOpen failed: %v\n", err)
	}
	defer func() {
		if err := s.Conn.DidClose(fname); err != nil {
			log.Printf("DidClose failed: %v\n", err)
		}
	}()
	return FormatFile(s.Conn, uri, w)
}

// FormatFile organizes import paths and then formats the file f.
func FormatFile(c *client.Conn, uri lsp.DocumentURI, f text.File) error {
	if c.Capabilities.CodeActionProvider {
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
			// TODO(fhs): check if uri is among the files edited?
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

// Rename renames the identifier at position pos to newname.
func Rename(c *client.Conn, pos *lsp.TextDocumentPositionParams, newname string) error {
	we, err := c.Rename(pos, newname)
	if err != nil {
		return err
	}
	return editWorkspace(we)
}

func editWorkspace(we *lsp.WorkspaceEdit) error {
	wins, err := acme.Windows()
	if err != nil {
		return errors.Wrapf(err, "failed to read list of acme index")
	}
	winid := make(map[string]int, len(wins))
	for _, info := range wins {
		winid[info.Name] = info.ID
	}

	for uri := range we.Changes {
		fname := text.ToPath(uri)
		if _, ok := winid[fname]; !ok {
			return fmt.Errorf("%v: not open in acme", fname)
		}
	}
	for uri, edits := range we.Changes {
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

// FormatOnPut watches for Put executed on an acme window and formats it using LSP.
func FormatOnPut(serverSet *client.ServerSet) {
	alog, err := acme.Log()
	if err != nil {
		panic(err)
	}
	defer alog.Close()
	for {
		ev, err := alog.Read()
		if err != nil {
			panic(err)
		}
		if ev.Op == "put" {
			if err = formatWin(serverSet, ev.ID); err != nil {
				log.Printf("formating window %v failed: %v\n", ev.ID, err)
			}
		}
	}
}

// ParseFlags adds some standard flags, parses all flags, and returns the server set and debug.
func ParseFlags(serverSet *client.ServerSet) (*client.ServerSet, bool) {
	var (
		userServers serverFlag
		dialServers serverFlag
	)

	debug := flag.Bool("debug", false, "turn on debugging prints")
	workspaces := flag.String("workspaces", "", "colon-separated list of initial workspace directories")
	flag.Var(&userServers, "server", `language server command for filename match (e.g. '\.go$:gopls')`)
	flag.Var(&dialServers, "dial", `language server address for filename match (e.g. '\.go$:localhost:4389')`)
	flag.Parse()

	if *debug {
		client.Debug = true
	}

	if serverSet == nil {
		serverSet = new(client.ServerSet)
	}
	if len(*workspaces) > 0 {
		serverSet.Workspaces = strings.Split(*workspaces, ":")
	}

	if len(userServers) > 0 {
		for _, sa := range userServers {
			serverSet.Register(sa.pattern, strings.Fields(sa.args))
		}
	}
	if len(dialServers) > 0 {
		for _, sa := range userServers {
			serverSet.RegisterDial(sa.pattern, sa.args)
		}
	}
	return serverSet, *debug
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
		return errors.New("bad flag value")
	}
	*sf = append(*sf, serverArgs{
		pattern: f[0],
		args:    f[1],
	})
	return nil
}
