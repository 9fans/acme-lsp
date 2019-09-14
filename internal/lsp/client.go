// Package lsp implements a general LSP client.
package lsp

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/fhs/acme-lsp/internal/golang_org_x_tools/jsonrpc2"
	"github.com/fhs/acme-lsp/internal/lsp/protocol"
	"github.com/fhs/acme-lsp/internal/lsp/text"
	"github.com/pkg/errors"
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

// clientHandler handles JSON-RPC requests and notifications.
type clientHandler struct {
	diagWriter DiagnosticsWriter
	diag       map[protocol.DocumentURI][]protocol.Diagnostic
	mu         sync.Mutex
}

func (h *clientHandler) ShowMessage(ctx context.Context, params *protocol.ShowMessageParams) error {
	log.Printf("LSP %v: %v\n", params.Type, params.Message)
	return nil
}

func (h *clientHandler) LogMessage(ctx context.Context, params *protocol.LogMessageParams) error {
	if params.Type == protocol.Error || params.Type == protocol.Warning || Debug {
		log.Printf("log: LSP %v: %v\n", params.Type, params.Message)
	}
	return nil
}

func (h *clientHandler) Event(context.Context, *interface{}) error {
	return nil
}

func (h *clientHandler) PublishDiagnostics(ctx context.Context, params *protocol.PublishDiagnosticsParams) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.diag[params.URI]) == 0 && len(params.Diagnostics) == 0 {
		return nil
	}
	h.diag[params.URI] = params.Diagnostics

	h.diagWriter.WriteDiagnostics(h.diag)
	return nil
}

func (h *clientHandler) WorkspaceFolders(context.Context) ([]protocol.WorkspaceFolder, error) {
	return nil, nil
}

func (h *clientHandler) Configuration(context.Context, *protocol.ParamConfig) ([]interface{}, error) {
	return nil, nil
}

func (h *clientHandler) RegisterCapability(context.Context, *protocol.RegistrationParams) error {
	return nil
}

func (h *clientHandler) UnregisterCapability(context.Context, *protocol.UnregistrationParams) error {
	return nil
}

func (h *clientHandler) ShowMessageRequest(context.Context, *protocol.ShowMessageRequestParams) (*protocol.MessageActionItem, error) {
	return nil, nil
}

func (h *clientHandler) ApplyEdit(context.Context, *protocol.ApplyWorkspaceEditParams) (*protocol.ApplyWorkspaceEditResponse, error) {
	return &protocol.ApplyWorkspaceEditResponse{Applied: false, FailureReason: "not implemented"}, nil
}

// Config contains LSP client configuration values.
type Config struct {
	DiagWriter DiagnosticsWriter // notification handler writes diagnostics here
	RootDir    string            // directory for RootURI
	Workspaces []string          // initial workspaces
}

// Client represents a LSP client connection.
type Client struct {
	server       protocol.Server
	ctx          context.Context
	Capabilities *protocol.ServerCapabilities
}

func New(conn net.Conn, cfg *Config) (*Client, error) {
	ctx := context.Background()
	stream := jsonrpc2.NewHeaderStream(conn, conn)
	ctx, rpc, server := protocol.NewClient(ctx, stream, &clientHandler{
		diagWriter: cfg.DiagWriter,
		diag:       make(map[protocol.DocumentURI][]protocol.Diagnostic),
	})
	go func() {
		err := rpc.Run(ctx)
		if err != nil {
			log.Printf("connection terminated: %v", err)
		}
	}()

	d, err := filepath.Abs(cfg.RootDir)
	if err != nil {
		return nil, err
	}
	params := &protocol.InitializeParams{
		RootURI: text.ToURI(d),
	}
	params.Capabilities.Workspace.WorkspaceFolders = true
	params.Capabilities.TextDocument.CodeAction.CodeActionLiteralSupport = new(protocol.CodeActionLiteralSupport)
	params.Capabilities.TextDocument.CodeAction.CodeActionLiteralSupport.CodeActionKind.ValueSet =
		[]protocol.CodeActionKind{protocol.SourceOrganizeImports}
	params.Capabilities.TextDocument.DocumentSymbol.HierarchicalDocumentSymbolSupport = true
	params.WorkspaceFolders, err = dirsToWorkspaceFolders(cfg.Workspaces)
	if err != nil {
		return nil, err
	}
	var result protocol.InitializeResult
	if err := rpc.Call(ctx, "initialize", params, &result); err != nil {
		return nil, errors.Wrap(err, "initialize failed")
	}
	if err := rpc.Notify(ctx, "initialized", &protocol.InitializedParams{}); err != nil {
		return nil, errors.Wrap(err, "initialized failed")
	}
	return &Client{
		server:       server,
		ctx:          ctx,
		Capabilities: &result.Capabilities,
	}, nil
}

func (c *Client) Close() error {
	// TODO(fhs): Cancel all outstanding requests?
	return nil
}

func (c *Client) Definition(pos *protocol.TextDocumentPositionParams) ([]protocol.Location, error) {
	return c.server.Definition(c.ctx, &protocol.DefinitionParams{
		TextDocumentPositionParams: *pos,
	})
}

func (c *Client) TypeDefinition(pos *protocol.TextDocumentPositionParams) ([]protocol.Location, error) {
	return c.server.TypeDefinition(c.ctx, &protocol.TypeDefinitionParams{
		TextDocumentPositionParams: *pos,
	})
}

func (c *Client) Implementation(pos *protocol.TextDocumentPositionParams) ([]protocol.Location, error) {
	return c.server.Implementation(c.ctx, &protocol.ImplementationParams{
		TextDocumentPositionParams: *pos,
	})
}

func (c *Client) Hover(pos *protocol.TextDocumentPositionParams, w io.Writer) error {
	hov, err := c.server.Hover(c.ctx, &protocol.HoverParams{
		TextDocumentPositionParams: *pos,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "%v\n", hov.Contents.Value)
	return nil
}

func (c *Client) References(pos *protocol.TextDocumentPositionParams, w io.Writer) error {
	loc, err := c.server.References(c.ctx, &protocol.ReferenceParams{
		TextDocumentPositionParams: *pos,
		Context: protocol.ReferenceContext{
			IncludeDeclaration: true,
		},
	})
	if err != nil {
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

func walkDocumentSymbols(syms []protocol.DocumentSymbol, depth int, f func(s *protocol.DocumentSymbol, depth int)) {
	for _, s := range syms {
		f(&s, depth)
		walkDocumentSymbols(s.Children, depth+1, f)
	}
}

func (c *Client) Symbols(uri protocol.DocumentURI, w io.Writer) error {
	// TODO(fhs): DocumentSymbol request can return either a
	// []DocumentSymbol (hierarchical) or []SymbolInformation (flat).
	// We only handle the hierarchical type below.

	// TODO(fhs): Make use of DocumentSymbol.Range to optionally filter out
	// symbols that aren't within current cursor position?

	syms, err := c.server.DocumentSymbol(c.ctx, &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: uri,
		},
	})
	if err != nil {
		return err
	}
	if len(syms) == 0 {
		fmt.Printf("No symbols found.\n")
		return nil
	}
	fmt.Printf("Symbols:\n")
	walkDocumentSymbols(syms, 0, func(s *protocol.DocumentSymbol, depth int) {
		loc := &protocol.Location{
			URI:   uri,
			Range: s.SelectionRange,
		}
		indent := strings.Repeat(" ", depth)
		fmt.Fprintf(w, "%v %v %v %v\n", indent, s.Kind, s.Name, s.Detail)
		fmt.Fprintf(w, "%v  %v\n", indent, LocationLink(loc))
	})
	return nil
}

func (c *Client) Completion(pos *protocol.TextDocumentPositionParams) ([]protocol.CompletionItem, error) {
	cl, err := c.server.Completion(c.ctx, &protocol.CompletionParams{
		TextDocumentPositionParams: *pos,
		Context:                    &protocol.CompletionContext{},
	})
	if err != nil {
		return nil, err
	}
	return cl.Items, nil
}

func (c *Client) SignatureHelp(pos *protocol.TextDocumentPositionParams, w io.Writer) error {
	sh, err := c.server.SignatureHelp(c.ctx, &protocol.SignatureHelpParams{
		TextDocumentPositionParams: *pos,
	})
	if err != nil {
		return err
	}
	for _, sig := range sh.Signatures {
		fmt.Fprintf(w, "%v\n", sig.Label)
		fmt.Fprintf(w, "%v\n", sig.Documentation)
	}
	return nil
}

func (c *Client) Rename(pos *protocol.TextDocumentPositionParams, newname string) (*protocol.WorkspaceEdit, error) {
	return c.server.Rename(c.ctx, &protocol.RenameParams{
		TextDocument: pos.TextDocument,
		Position:     pos.Position,
		NewName:      newname,
	})
}

func (c *Client) Format(uri protocol.DocumentURI) ([]protocol.TextEdit, error) {
	return c.server.Formatting(c.ctx, &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: uri,
		},
	})
}

func (c *Client) OrganizeImports(uri protocol.DocumentURI) ([]protocol.CodeAction, error) {
	return c.server.CodeAction(c.ctx, &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: uri,
		},
		Range: protocol.Range{},
		Context: protocol.CodeActionContext{
			Diagnostics: nil,
			Only:        []protocol.CodeActionKind{protocol.SourceOrganizeImports},
		},
	})
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

func (c *Client) DidOpen(filename string, body []byte) error {
	return c.server.DidOpen(c.ctx, &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        text.ToURI(filename),
			LanguageID: fileLanguage(filename),
			Version:    0,
			Text:       string(body),
		},
	})
}

func (c *Client) DidClose(filename string) error {
	return c.server.DidClose(c.ctx, &protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: text.ToURI(filename),
		},
	})
}

func (c *Client) DidSave(filename string) error {
	return c.server.DidSave(c.ctx, &protocol.DidSaveTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{
				URI: text.ToURI(filename),
			},
			// TODO(fhs): add text field for includeText option
		},
	})
}

func (c *Client) DidChange(filename string, body []byte) error {
	return c.server.DidChange(c.ctx, &protocol.DidChangeTextDocumentParams{
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
	})
}

func (c *Client) DidChangeWorkspaceFolders(addedDirs, removedDirs []string) error {
	added, err := dirsToWorkspaceFolders(addedDirs)
	if err != nil {
		return err
	}
	removed, err := dirsToWorkspaceFolders(removedDirs)
	if err != nil {
		return err
	}
	return c.server.DidChangeWorkspaceFolders(c.ctx, &protocol.DidChangeWorkspaceFoldersParams{
		Event: protocol.WorkspaceFoldersChangeEvent{
			Added:   added,
			Removed: removed,
		},
	})
}

func (c *Client) ProvidesCodeAction(kind protocol.CodeActionKind) bool {
	switch ap := c.Capabilities.CodeActionProvider.(type) {
	case bool:
		return ap
	case map[string]interface{}:
		opt, err := protocol.ToCodeActionOptions(ap)
		if err != nil {
			log.Printf("failed to decode CodeActionOptions: %v", err)
			return false
		}
		for _, k := range opt.CodeActionKinds {
			if k == kind {
				return true
			}
		}
	}
	return false
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
