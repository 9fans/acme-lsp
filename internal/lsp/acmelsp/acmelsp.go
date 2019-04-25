// Package acmelsp defines helper functions for implementation of acme-lsp commands.
package acmelsp

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"9fans.net/go/acme"
	"9fans.net/go/plan9"
	p9client "9fans.net/go/plan9/client"
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
	srv, err := ss.StartForFile(fname)
	if err != nil {
		return nil, errors.Wrap(err, "cound not start language server")
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
	return PlumbDefinition(c.conn, c.pos)
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

// PlumbDefinition sends the location of where the identifier at positon pos is defined to the plumber.
func PlumbDefinition(c *client.Conn, pos *lsp.TextDocumentPositionParams) error {
	p, err := plumb.Open("send", plan9.OWRITE)
	if err != nil {
		return errors.Wrap(err, "failed to open plumber")
	}
	defer p.Close()
	locations, err := c.Definition(pos)
	if err != nil {
		return err
	}
	for _, loc := range locations {
		err := plumbLocation(p, &loc)
		if err != nil {
			return errors.Wrap(err, "failed to plumb location")
		}
	}
	return nil
}

func plumbLocation(p *p9client.Fid, loc *lsp.Location) error {
	fn := text.ToPath(loc.URI)
	a := fmt.Sprintf("%v:%v", fn, loc.Range.Start)

	m := &plumb.Message{
		Src:  "L",
		Dst:  "edit",
		Dir:  "/",
		Type: "text",
		Data: []byte(a),
	}
	return m.Send(p)
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
	s, err := serverSet.StartForFile(fname)
	if err != nil {
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

// FormatFile formats the file f.
func FormatFile(c *client.Conn, uri lsp.DocumentURI, f text.File) error {
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
func ParseFlags() (*client.ServerSet, bool) {
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

	var serverSet client.ServerSet
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
	return &serverSet, *debug
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
