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
}

func NewRemoteCmd(server proxy.Server, winid int) *RemoteCmd {
	return &RemoteCmd{
		server: server,
		winid:  winid,
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

func (rc *RemoteCmd) References(ctx context.Context, w io.Writer) error {
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
		fmt.Fprintf(w, "No references found.\n")
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
	fmt.Fprintf(w, "References:\n")
	for _, l := range loc {
		fmt.Fprintf(w, " %v\n", lsp.LocationLink(&l))
	}
	return nil
}
