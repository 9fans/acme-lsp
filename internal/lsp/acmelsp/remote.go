package acmelsp

import (
	"context"

	"github.com/fhs/acme-lsp/internal/acmeutil"
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
