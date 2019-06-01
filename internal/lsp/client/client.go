package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/fhs/acme-lsp/internal/lsp/protocol"
	"github.com/fhs/acme-lsp/internal/lsp/text"
	"github.com/pkg/errors"
	"github.com/sourcegraph/jsonrpc2"
)

var Debug = false

func LocationLink(l *protocol.Location) string {
	p := text.ToPath(l.URI)
	return fmt.Sprintf("%s:%v:%v-%v:%v", p,
		l.Range.Start.Line+1, l.Range.Start.Character+1,
		l.Range.End.Line+1, l.Range.End.Character+1)
}

type DiagnosticsWriter interface {
	WriteDiagnostics(map[protocol.DocumentURI][]protocol.Diagnostic) error
}

var _ = (jsonrpc2.Handler)(&handler{})

// handler handles JSON-RPC requests and notifications.
// Diagnostics and other messages sent by the server are printed to writer w.
type handler struct {
	w io.Writer

	diagWriter DiagnosticsWriter
	diag       map[protocol.DocumentURI][]protocol.Diagnostic
	mu         sync.Mutex
}

func (h *handler) Printf(format string, a ...interface{}) (n int, err error) {
	return fmt.Fprintf(h.w, format, a...)
}

func (h *handler) updateDiagnostics(params *protocol.PublishDiagnosticsParams) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.diag[params.URI]) == 0 && len(params.Diagnostics) == 0 {
		return
	}
	h.diag[params.URI] = params.Diagnostics

	h.diagWriter.WriteDiagnostics(h.diag)
}

func (h *handler) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	if strings.HasPrefix(req.Method, "$/") {
		// Ignore server dependent notifications
		if Debug {
			h.Printf("Handle: got request %#v\n", req)
		}
		return
	}
	switch req.Method {
	case "textDocument/publishDiagnostics":
		var params protocol.PublishDiagnosticsParams
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			h.Printf("diagnostics unmarshal failed: %v\n", err)
			return
		}
		h.updateDiagnostics(&params)
	case "window/showMessage":
		var params protocol.ShowMessageParams
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			h.Printf("window/showMessage unmarshal failed: %v\n", err)
			return
		}
		h.Printf("LSP %v: %v\n", params.Type, params.Message)
	case "window/logMessage":
		var params protocol.LogMessageParams
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			h.Printf("window/logMessage unmarshal failed: %v\n", err)
			return
		}
		if params.Type != protocol.Log || Debug {
			h.Printf("log: LSP %v: %v\n", params.Type, params.Message)
		}

	default:
		h.Printf("Handle: got request %#v\n", req)
	}
}

type Conn struct {
	rpc          *jsonrpc2.Conn
	ctx          context.Context
	Capabilities *protocol.ServerCapabilities
}

func New(conn net.Conn, w io.Writer, diagWriter DiagnosticsWriter, rootdir string, workspaces []string) (*Conn, error) {
	ctx := context.Background()
	stream := jsonrpc2.NewBufferedStream(conn, jsonrpc2.VSCodeObjectCodec{})
	rpc := jsonrpc2.NewConn(ctx, stream, &handler{
		w:          w,
		diagWriter: diagWriter,
		diag:       make(map[protocol.DocumentURI][]protocol.Diagnostic),
	})

	d, err := filepath.Abs(rootdir)
	if err != nil {
		return nil, err
	}
	params := &protocol.InitializeParams{
		RootURI: text.ToURI(d),
		Capabilities: protocol.ClientCapabilities{
			Workspace: protocol.WorkspaceClientCapabilities{
				WorkspaceFolders: true,
			},
		},
	}
	params.WorkspaceFolders, err = dirsToWorkspaceFolders(workspaces)
	if err != nil {
		return nil, err
	}
	var result protocol.InitializeResult
	if err := rpc.Call(ctx, "initialize", params, &result); err != nil {
		return nil, errors.Wrap(err, "initialize failed")
	}
	return &Conn{
		rpc:          rpc,
		ctx:          ctx,
		Capabilities: &result.Capabilities,
	}, nil
}

func (c *Conn) Close() error {
	return c.rpc.Close()
}

func (c *Conn) Definition(pos *protocol.TextDocumentPositionParams) ([]protocol.Location, error) {
	loc := make([]protocol.Location, 1)
	if err := c.rpc.Call(c.ctx, "textDocument/definition", pos, &loc); err != nil {
		return nil, err
	}
	return loc, nil
}

func (c *Conn) TypeDefinition(pos *protocol.TextDocumentPositionParams) ([]protocol.Location, error) {
	loc := make([]protocol.Location, 1)
	if err := c.rpc.Call(c.ctx, "textDocument/typeDefinition", pos, &loc); err != nil {
		return nil, err
	}
	return loc, nil
}

func (c *Conn) Implementation(pos *protocol.TextDocumentPositionParams) ([]protocol.Location, error) {
	loc := make([]protocol.Location, 1)
	if err := c.rpc.Call(c.ctx, "textDocument/implementation", pos, &loc); err != nil {
		return nil, err
	}
	return loc, nil
}

func (c *Conn) Hover(pos *protocol.TextDocumentPositionParams, w io.Writer) error {
	var hov protocol.Hover
	if err := c.rpc.Call(c.ctx, "textDocument/hover", pos, &hov); err != nil {
		return err
	}
	fmt.Fprintf(w, "%v\n", hov.Contents.Value)
	return nil
}

func (c *Conn) References(pos *protocol.TextDocumentPositionParams, w io.Writer) error {
	rp := &protocol.ReferenceParams{
		TextDocumentPositionParams: *pos,
		Context: protocol.ReferenceContext{
			IncludeDeclaration: true,
		},
	}
	loc := make([]protocol.Location, 1)
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
		fmt.Fprintf(w, " %v\n", LocationLink(&l))
	}
	return nil
}

func (c *Conn) Symbols(uri protocol.DocumentURI, w io.Writer) error {
	params := &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: uri,
		},
	}
	var syms []protocol.SymbolInformation
	if err := c.rpc.Call(c.ctx, "textDocument/documentSymbol", params, &syms); err != nil {
		return err
	}
	if len(syms) == 0 {
		fmt.Printf("No symbols found.\n")
		return nil
	}
	fmt.Printf("Symbols:\n")
	for _, s := range syms {
		fmt.Fprintf(w, " %v %v %v %v\n", s.ContainerName, s.Name, s.Kind, LocationLink(&s.Location))
	}
	return nil
}

func (c *Conn) Completion(pos *protocol.TextDocumentPositionParams, w io.Writer) error {
	comp := &protocol.CompletionParams{
		TextDocumentPositionParams: *pos,
		Context:                    protocol.CompletionContext{},
	}
	var cl protocol.CompletionList
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

func (c *Conn) SignatureHelp(pos *protocol.TextDocumentPositionParams, w io.Writer) error {
	var sh protocol.SignatureHelp
	if err := c.rpc.Call(c.ctx, "textDocument/signatureHelp", pos, &sh); err != nil {
		return err
	}
	for _, sig := range sh.Signatures {
		fmt.Fprintf(w, "%v\n", sig.Label)
		fmt.Fprintf(w, "%v\n", sig.Documentation)
	}
	return nil
}

func (c *Conn) Rename(pos *protocol.TextDocumentPositionParams, newname string) (*protocol.WorkspaceEdit, error) {
	params := &protocol.RenameParams{
		TextDocument: pos.TextDocument,
		Position:     pos.Position,
		NewName:      newname,
	}
	var we protocol.WorkspaceEdit
	if err := c.rpc.Call(c.ctx, "textDocument/rename", params, &we); err != nil {
		return nil, err
	}
	return &we, nil
}

func (c *Conn) Format(uri protocol.DocumentURI) ([]protocol.TextEdit, error) {
	params := &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: uri,
		},
	}
	var edits []protocol.TextEdit
	if err := c.rpc.Call(c.ctx, "textDocument/formatting", params, &edits); err != nil {
		return nil, err
	}
	return edits, nil
}

func (c *Conn) OrganizeImports(uri protocol.DocumentURI) ([]protocol.CodeAction, error) {
	params := &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: uri,
		},
		Range: protocol.Range{},
		Context: protocol.CodeActionContext{
			Diagnostics: nil,
			Only:        []protocol.CodeActionKind{protocol.SourceOrganizeImports},
		},
	}
	var actions []protocol.CodeAction
	if err := c.rpc.Call(c.ctx, "textDocument/codeAction", params, &actions); err != nil {
		return nil, err
	}
	return actions, nil
}

func fileLanguage(filename string) string {
	lang := filepath.Ext(filename)
	if len(lang) == 0 {
		return lang
	}
	if lang[0] == '.' {
		lang = lang[1:]
	}
	switch lang {
	case "py":
		lang = "python"
	}
	return lang
}

func (c *Conn) DidOpen(filename string, body []byte) error {
	params := &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        text.ToURI(filename),
			LanguageID: fileLanguage(filename),
			Version:    0,
			Text:       string(body),
		},
	}
	return c.rpc.Notify(c.ctx, "textDocument/didOpen", params)
}

func (c *Conn) DidClose(filename string) error {
	params := &protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: text.ToURI(filename),
		},
	}
	return c.rpc.Notify(c.ctx, "textDocument/didClose", params)
}

func (c *Conn) DidChange(filename string, body []byte) error {
	params := &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{
				URI: text.ToURI(filename),
			},
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			{
				Text: string(body),
			},
		},
	}
	return c.rpc.Notify(c.ctx, "textDocument/didChange", params)
}

func (c *Conn) DidChangeWorkspaceFolders(addedDirs, removedDirs []string) error {
	added, err := dirsToWorkspaceFolders(addedDirs)
	if err != nil {
		return err
	}
	removed, err := dirsToWorkspaceFolders(removedDirs)
	if err != nil {
		return err
	}
	params := &protocol.DidChangeWorkspaceFoldersParams{
		Event: protocol.WorkspaceFoldersChangeEvent{
			Added:   added,
			Removed: removed,
		},
	}
	return c.rpc.Notify(c.ctx, "workspace/didChangeWorkspaceFolders", params)
}

func dirsToWorkspaceFolders(dirs []string) ([]protocol.WorkspaceFolder, error) {
	var workspaces []protocol.WorkspaceFolder
	for _, d := range dirs {
		d, err := filepath.Abs(d)
		if err != nil {
			return nil, err
		}
		workspaces = append(workspaces, protocol.WorkspaceFolder{
			URI:  text.ToURI(d),
			Name: d,
		})
	}
	return workspaces, nil
}
