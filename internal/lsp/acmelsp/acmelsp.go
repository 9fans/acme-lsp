// Package acmelsp defines helper functions for implementation of acme-lsp commands.
package acmelsp

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	"9fans.net/go/acme"
	"9fans.net/go/plan9"
	"9fans.net/go/plumb"
	"github.com/fhs/acme-lsp/internal/acmeutil"
	"github.com/fhs/acme-lsp/internal/lsp"
	"github.com/fhs/acme-lsp/internal/lsp/acmelsp/config"
	"github.com/fhs/acme-lsp/internal/lsp/protocol"
	"github.com/fhs/acme-lsp/internal/lsp/text"
	"github.com/pkg/errors"
)

func CurrentWindowRemoteCmd(ss *lsp.ServerSet, fm *FileManager) (*RemoteCmd, error) {
	id, err := strconv.Atoi(os.Getenv("winid"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse $winid")
	}
	return WindowRemoteCmd(ss, fm, id)
}

func WindowRemoteCmd(ss *lsp.ServerSet, fm *FileManager, winid int) (*RemoteCmd, error) {
	w, err := acmeutil.OpenWin(winid)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to to open window %v", winid)
	}
	defer w.CloseFiles()

	_, fname, err := text.DocumentURI(w)
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

	return NewRemoteCmd(srv.Client, winid), nil
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

type FormatServer interface {
	InitializeResult(context.Context, *protocol.TextDocumentIdentifier) (*protocol.InitializeResult, error)
	DidChange(context.Context, *protocol.DidChangeTextDocumentParams) error
	Formatting(context.Context, *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error)
	CodeAction(context.Context, *protocol.CodeActionParams) ([]protocol.CodeAction, error)
}

// OrganizeImportsAndFormat organizes import paths and then formats the file f.
func OrganizeImportsAndFormat(ctx context.Context, server FormatServer, doc *protocol.TextDocumentIdentifier, f text.File) error {
	initres, err := server.InitializeResult(ctx, doc)
	if err != nil {
		return err
	}

	if lsp.ServerProvidesCodeAction(&initres.Capabilities, protocol.SourceOrganizeImports) {
		actions, err := server.CodeAction(ctx, &protocol.CodeActionParams{
			TextDocument: *doc,
			Range:        protocol.Range{},
			Context: protocol.CodeActionContext{
				Diagnostics: nil,
				Only:        []protocol.CodeActionKind{protocol.SourceOrganizeImports},
			},
		})
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
			server.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
				TextDocument: protocol.VersionedTextDocumentIdentifier{
					TextDocumentIdentifier: *doc,
				},
				ContentChanges: []protocol.TextDocumentContentChangeEvent{
					{
						Text: string(b),
					},
				},
			})
			if err != nil {
				return err
			}
		}
	}
	edits, err := server.Formatting(ctx, &protocol.DocumentFormattingParams{
		TextDocument: *doc,
	})
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

// NewServerSet creates a new server set from config.
func NewServerSet(cfg *config.Config) (*lsp.ServerSet, error) {
	if cfg.Verbose {
		lsp.Debug = true
	}

	serverSet := lsp.NewServerSet(DefaultConfig())

	if len(cfg.WorkspaceDirectories) > 0 {
		folders, err := lsp.DirsToWorkspaceFolders(cfg.WorkspaceDirectories)
		if err != nil {
			return nil, err
		}
		serverSet.InitWorkspaces(folders)
	}
	for _, ls := range cfg.LegacyLanguageServers {
		switch {
		case len(ls.Command) > 0:
			serverSet.Register(ls.Pattern, ls.Command)
		case len(ls.DialAddress) > 0:
			serverSet.RegisterDial(ls.Pattern, ls.DialAddress)
		default:
			return nil, fmt.Errorf("invalid legacy server flag value")
		}
	}
	return serverSet, nil
}

func DefaultConfig() *lsp.Config {
	return &lsp.Config{
		DiagWriter: NewDiagnosticsWriter(),
		RootDir:    "/",
		Workspaces: nil,
	}
}
