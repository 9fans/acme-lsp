package main

import (
	"fmt"

	p9client "9fans.net/go/plan9/client"
	"9fans.net/go/plumb"
	"github.com/fhs/acme-lsp/internal/lsp"
	"github.com/fhs/acme-lsp/internal/lsp/client"
	"github.com/pkg/errors"
)

func plumbLocation(p *p9client.Fid, loc *lsp.Location) error {
	fn := uriToFilename(loc.URI)
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

func renameInEditor(c *client.Conn, pos *lsp.TextDocumentPositionParams, newname string) error {
	we, err := c.Rename(pos, newname)
	if err != nil {
		return err
	}
	return applyAcmeEdits(we)
}

func formatInEditor(c *client.Conn, uri lsp.DocumentURI, e editor) error {
	edits, err := c.Format(uri)
	if err != nil {
		return err
	}
	if err := e.Edit(edits); err != nil {
		return errors.Wrapf(err, "failed to apply edits")
	}
	return nil
}
