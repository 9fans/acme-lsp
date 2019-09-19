package acmelsp

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/fhs/acme-lsp/internal/acmeutil"
	"github.com/fhs/acme-lsp/internal/lsp"
	"github.com/fhs/acme-lsp/internal/lsp/protocol"
	"github.com/fhs/acme-lsp/internal/lsp/proxy"
	"github.com/fhs/acme-lsp/internal/lsp/text"
	"github.com/pkg/errors"
)

type RemoteCmd struct {
	server proxy.Server
	winid  int
	Stdout io.Writer
	Stderr io.Writer
}

func NewRemoteCmd(server proxy.Server, winid int) *RemoteCmd {
	return &RemoteCmd{
		server: server,
		winid:  winid,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

func (rc *RemoteCmd) getPosition() (pos *protocol.TextDocumentPositionParams, filename string, err error) {
	w, err := acmeutil.OpenWin(rc.winid)
	if err != nil {
		return nil, "", errors.Wrapf(err, "failed to to open window %v", rc.winid)
	}
	defer w.CloseFiles()

	return text.Position(w)
}

func (rc *RemoteCmd) Completion(ctx context.Context, edit bool) error {
	w, err := acmeutil.OpenWin(rc.winid)
	if err != nil {
		return err
	}
	defer w.CloseFiles()

	pos, _, err := text.Position(w)
	if err != nil {
		return err
	}
	result, err := rc.server.Completion(ctx, &protocol.CompletionParams{
		TextDocumentPositionParams: *pos,
		Context:                    &protocol.CompletionContext{},
	})
	if err != nil {
		return err
	}
	if edit && len(result.Items) == 1 {
		textEdit := result.Items[0].TextEdit
		if textEdit == nil {
			// TODO(fhs): Use insertText or label instead.
			return fmt.Errorf("nil TextEdit in completion item")
		}
		if err := text.Edit(w, []protocol.TextEdit{*textEdit}); err != nil {
			return errors.Wrapf(err, "failed to apply completion edit")
		}
		return nil
	}
	printCompletionItems(os.Stdout, result.Items)
	return nil
}

func (rc *RemoteCmd) Definition(ctx context.Context) error {
	pos, _, err := rc.getPosition()
	if err != nil {
		return err
	}
	locations, err := rc.server.Definition(ctx, &protocol.DefinitionParams{
		TextDocumentPositionParams: *pos,
	})
	if err != nil {
		return err
	}
	return PlumbLocations(locations)
}

func (rc *RemoteCmd) Hover(ctx context.Context) error {
	pos, _, err := rc.getPosition()
	if err != nil {
		return err
	}
	hov, err := rc.server.Hover(ctx, &protocol.HoverParams{
		TextDocumentPositionParams: *pos,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(rc.Stdout, "%v\n", hov.Contents.Value)
	return nil
}

func (rc *RemoteCmd) References(ctx context.Context) error {
	pos, _, err := rc.getPosition()
	if err != nil {
		return err
	}
	loc, err := rc.server.References(ctx, &protocol.ReferenceParams{
		TextDocumentPositionParams: *pos,
		Context: protocol.ReferenceContext{
			IncludeDeclaration: true,
		},
	})
	if err != nil {
		return err
	}
	if len(loc) == 0 {
		fmt.Fprintf(rc.Stderr, "No references found.\n")
		return nil
	}
	sort.Slice(loc, func(i, j int) bool {
		a := loc[i]
		b := loc[j]
		n := strings.Compare(string(a.URI), string(b.URI))
		if n == 0 {
			m := a.Range.Start.Line - b.Range.Start.Line
			if m == 0 {
				return a.Range.Start.Character < b.Range.Start.Character
			}
			return m < 0
		}
		return n < 0
	})
	for _, l := range loc {
		fmt.Fprintf(rc.Stdout, "%v\n", lsp.LocationLink(&l))
	}
	return nil
}

func (rc *RemoteCmd) Rename(ctx context.Context, newname string) error {
	pos, _, err := rc.getPosition()
	if err != nil {
		return err
	}
	we, err := rc.server.Rename(ctx, &protocol.RenameParams{
		TextDocument: pos.TextDocument,
		Position:     pos.Position,
		NewName:      newname,
	})
	if err != nil {
		return err
	}
	return editWorkspace(we)
}

func (rc *RemoteCmd) SignatureHelp(ctx context.Context) error {
	pos, _, err := rc.getPosition()
	if err != nil {
		return err
	}
	sh, err := rc.server.SignatureHelp(ctx, &protocol.SignatureHelpParams{
		TextDocumentPositionParams: *pos,
	})
	if err != nil {
		return err
	}
	for _, sig := range sh.Signatures {
		fmt.Fprintf(rc.Stdout, "%v\n", sig.Label)
		fmt.Fprintf(rc.Stdout, "%v\n", sig.Documentation)
	}
	return nil
}

func (rc *RemoteCmd) DocumentSymbol(ctx context.Context) error {
	win, err := acmeutil.OpenWin(rc.winid)
	if err != nil {
		return err
	}
	defer win.CloseFiles()

	uri, _, err := text.DocumentURI(win)
	if err != nil {
		return err
	}

	// TODO(fhs): DocumentSymbol request can return either a
	// []DocumentSymbol (hierarchical) or []SymbolInformation (flat).
	// We only handle the hierarchical type below.

	// TODO(fhs): Make use of DocumentSymbol.Range to optionally filter out
	// symbols that aren't within current cursor position?

	syms, err := rc.server.DocumentSymbol(ctx, &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: uri,
		},
	})
	if err != nil {
		return err
	}
	if len(syms) == 0 {
		fmt.Fprintf(rc.Stderr, "No symbols found.\n")
		return nil
	}
	walkDocumentSymbols(syms, 0, func(s *protocol.DocumentSymbol, depth int) {
		loc := &protocol.Location{
			URI:   uri,
			Range: s.SelectionRange,
		}
		indent := strings.Repeat(" ", depth)
		fmt.Fprintf(rc.Stdout, "%v%v %v %v\n", indent, s.Kind, s.Name, s.Detail)
		fmt.Fprintf(rc.Stdout, "%v %v\n", indent, lsp.LocationLink(loc))
	})
	return nil
}

func (rc *RemoteCmd) TypeDefinition(ctx context.Context) error {
	pos, _, err := rc.getPosition()
	if err != nil {
		return err
	}
	locations, err := rc.server.TypeDefinition(ctx, &protocol.TypeDefinitionParams{
		TextDocumentPositionParams: *pos,
	})
	if err != nil {
		return err
	}
	return PlumbLocations(locations)
}

func walkDocumentSymbols(syms []protocol.DocumentSymbol, depth int, f func(s *protocol.DocumentSymbol, depth int)) {
	for _, s := range syms {
		f(&s, depth)
		walkDocumentSymbols(s.Children, depth+1, f)
	}
}
