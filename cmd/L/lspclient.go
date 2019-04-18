package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"path/filepath"
	"sort"
	"strings"

	"9fans.net/go/plan9/client"
	"9fans.net/go/plumb"
	"github.com/fhs/acme-lsp/internal/lsp"
	"github.com/pkg/errors"
	"github.com/sourcegraph/jsonrpc2"
)

type lspHandler struct{}

func (h *lspHandler) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	if strings.HasPrefix(req.Method, "$/") {
		// Ignore server dependent notifications
		if *debug {
			fmt.Printf("Handle: got request %#v\n", req)
		}
		return
	}
	switch req.Method {
	case "textDocument/publishDiagnostics":
		var params lsp.PublishDiagnosticsParams
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			log.Printf("diagnostics unmarshal failed: %v\n", err)
			return
		}
		if len(params.Diagnostics) > 0 {
			fmt.Printf("LSP Diagnostics:\n")
		}
		for _, diag := range params.Diagnostics {
			loc := &lsp.Location{
				URI:   params.URI,
				Range: diag.Range,
			}
			fmt.Printf(" %v: %v\n", locToLink(loc), diag.Message)
		}
	case "window/showMessage":
		var params lsp.ShowMessageParams
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			log.Printf("window/showMessage unmarshal failed: %v\n", err)
			return
		}
		fmt.Printf("LSP %v: %v\n", params.Type, params.Message)
	case "window/logMessage":
		var params lsp.LogMessageParams
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			log.Printf("window/logMessage unmarshal failed: %v\n", err)
			return
		}
		fmt.Printf("LSP %v: %v\n", params.Type, params.Message)

	default:
		fmt.Printf("Handle: got request %#v\n", req)
	}
}

type lspClient struct {
	rpc *jsonrpc2.Conn
	ctx context.Context
}

func newLSPClient(conn net.Conn, rootdir string) (*lspClient, error) {
	ctx := context.Background()
	stream := jsonrpc2.NewBufferedStream(conn, jsonrpc2.VSCodeObjectCodec{})
	rpc := jsonrpc2.NewConn(ctx, stream, &lspHandler{})

	d, err := filepath.Abs(rootdir)
	if err != nil {
		return nil, err
	}
	initp := &lsp.InitializeParams{
		RootURI: filenameToURI(d),
	}
	initr := &lsp.InitializeResult{}
	if err := rpc.Call(ctx, "initialize", initp, initr); err != nil {
		return nil, errors.Wrap(err, "initialize failed")
	}
	return &lspClient{
		rpc: rpc,
		ctx: ctx,
	}, nil
}

func (c *lspClient) Close() error {
	return c.rpc.Close()
}

func plumbLocation(p *client.Fid, loc *lsp.Location) error {
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

func locToLink(l *lsp.Location) string {
	p := uriToFilename(l.URI)
	return fmt.Sprintf("%s:%v:%v-%v:%v", p,
		l.Range.Start.Line+1, l.Range.Start.Character+1,
		l.Range.End.Line+1, l.Range.End.Character+1)
}

func (c *lspClient) Definition(pos *lsp.TextDocumentPositionParams) ([]lsp.Location, error) {
	loc := make([]lsp.Location, 1)
	if err := c.rpc.Call(c.ctx, "textDocument/definition", pos, &loc); err != nil {
		return nil, err
	}
	return loc, nil
}

func (c *lspClient) Hover(pos *lsp.TextDocumentPositionParams, w io.Writer) error {
	var hov lsp.Hover
	if err := c.rpc.Call(c.ctx, "textDocument/hover", pos, &hov); err != nil {
		return err
	}
	fmt.Fprintf(w, "%v\n", hov.Contents.Value)
	return nil
}

func (c *lspClient) References(pos *lsp.TextDocumentPositionParams, w io.Writer) error {
	rp := &lsp.ReferenceParams{
		TextDocumentPositionParams: *pos,
		Context: lsp.ReferenceContext{
			IncludeDeclaration: true,
		},
	}
	loc := make([]lsp.Location, 1)
	if err := c.rpc.Call(c.ctx, "textDocument/references", rp, &loc); err != nil {
		return err
	}
	if len(loc) == 0 {
		fmt.Printf("No references found.\n")
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
	fmt.Printf("References:\n")
	for _, l := range loc {
		fmt.Fprintf(w, " %v\n", locToLink(&l))
	}
	return nil
}

func (c *lspClient) Symbols(uri lsp.DocumentURI, w io.Writer) error {
	params := &lsp.DocumentSymbolParams{
		TextDocument: lsp.TextDocumentIdentifier{
			URI: uri,
		},
	}
	var syms []lsp.SymbolInformation
	if err := c.rpc.Call(c.ctx, "textDocument/documentSymbol", params, &syms); err != nil {
		return err
	}
	if len(syms) == 0 {
		fmt.Printf("No symbols found.\n")
		return nil
	}
	fmt.Printf("Symbols:\n")
	for _, s := range syms {
		fmt.Fprintf(w, " %v %v %v %v\n", s.ContainerName, s.Name, s.Kind, locToLink(&s.Location))
	}
	return nil
}

func (c *lspClient) Completion(pos *lsp.TextDocumentPositionParams, w io.Writer) error {
	comp := &lsp.CompletionParams{
		TextDocumentPositionParams: *pos,
		Context:                    lsp.CompletionContext{},
	}
	var cl lsp.CompletionList
	if err := c.rpc.Call(c.ctx, "textDocument/completion", comp, &cl); err != nil {
		return err
	}
	if len(cl.Items) == 0 {
		fmt.Fprintf(w, "no completion\n")
	}
	for _, item := range cl.Items {
		fmt.Fprintf(w, "%v %v\n", item.Label, item.Detail)
	}
	return nil
}

func (c *lspClient) SignatureHelp(pos *lsp.TextDocumentPositionParams, w io.Writer) error {
	var sh lsp.SignatureHelp
	if err := c.rpc.Call(c.ctx, "textDocument/signatureHelp", pos, &sh); err != nil {
		return err
	}
	for _, sig := range sh.Signatures {
		fmt.Fprintf(w, "%v\n", sig.Label)
		fmt.Fprintf(w, "%v\n", sig.Documentation)
	}
	return nil
}

func (c *lspClient) Rename(pos *lsp.TextDocumentPositionParams, newname string) error {
	params := &lsp.RenameParams{
		TextDocument: pos.TextDocument,
		Position:     pos.Position,
		NewName:      newname,
	}
	var we lsp.WorkspaceEdit
	if err := c.rpc.Call(c.ctx, "textDocument/rename", params, &we); err != nil {
		return err
	}
	return applyAcmeEdits(&we)
}

func (c *lspClient) Format(uri lsp.DocumentURI) ([]lsp.TextEdit, error) {
	params := &lsp.DocumentFormattingParams{
		TextDocument: lsp.TextDocumentIdentifier{
			URI: uri,
		},
	}
	var edits []lsp.TextEdit
	if err := c.rpc.Call(c.ctx, "textDocument/formatting", params, &edits); err != nil {
		return nil, err
	}
	return edits, nil
}

func (c *lspClient) FormatInEditor(uri lsp.DocumentURI, e editor) error {
	edits, err := c.Format(uri)
	if err != nil {
		return err
	}
	if err := e.Edit(edits); err != nil {
		return errors.Wrapf(err, "failed to apply edits")
	}
	return nil
}

func (c *lspClient) DidOpen(filename string, body []byte) error {
	lang := filepath.Ext(filename)
	switch lang {
	case "py":
		lang = "python"
	}
	params := &lsp.DidOpenTextDocumentParams{
		TextDocument: lsp.TextDocumentItem{
			URI:        filenameToURI(filename),
			LanguageID: lang,
			Version:    0,
			Text:       string(body),
		},
	}
	return c.rpc.Notify(c.ctx, "textDocument/didOpen", params)
}

func (c *lspClient) DidClose(filename string) error {
	params := &lsp.DidCloseTextDocumentParams{
		TextDocument: lsp.TextDocumentIdentifier{
			URI: filenameToURI(filename),
		},
	}
	return c.rpc.Notify(c.ctx, "textDocument/didClose", params)
}
