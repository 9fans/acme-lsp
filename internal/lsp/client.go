// Package lsp implements a general LSP client.
package lsp

import (
	"context"
	"fmt"
	"log"
	"net"
	"path/filepath"
	"sync"

	"github.com/fhs/acme-lsp/internal/golang_org_x_tools/jsonrpc2"
	"github.com/fhs/acme-lsp/internal/lsp/acmelsp/config"
	"github.com/fhs/acme-lsp/internal/lsp/protocol"
	"github.com/fhs/acme-lsp/internal/lsp/proxy"
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
	*config.Config
	DiagWriter DiagnosticsWriter          // notification handler writes diagnostics here
	Workspaces []protocol.WorkspaceFolder // initial workspace folders
}

// Client represents a LSP client connection.
type Client struct {
	protocol.Server
	initializeResult *protocol.InitializeResult
}

func NewClient(conn net.Conn, cfg *Config) (*Client, error) {
	c := &Client{}
	if err := c.init(conn, cfg); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Client) init(conn net.Conn, cfg *Config) error {
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

	d, err := filepath.Abs(cfg.RootDirectory)
	if err != nil {
		return err
	}
	params := &protocol.InitializeParams{
		RootURI: text.ToURI(d),
	}
	params.Capabilities.Workspace.WorkspaceFolders = true
	params.Capabilities.TextDocument.CodeAction.CodeActionLiteralSupport = new(protocol.CodeActionLiteralSupport)
	params.Capabilities.TextDocument.CodeAction.CodeActionLiteralSupport.CodeActionKind.ValueSet =
		[]protocol.CodeActionKind{protocol.SourceOrganizeImports}
	params.Capabilities.TextDocument.DocumentSymbol.HierarchicalDocumentSymbolSupport = true
	params.WorkspaceFolders = cfg.Workspaces

	var result protocol.InitializeResult
	if err := rpc.Call(ctx, "initialize", params, &result); err != nil {
		return errors.Wrap(err, "initialize failed")
	}
	if err := rpc.Notify(ctx, "initialized", &protocol.InitializedParams{}); err != nil {
		return errors.Wrap(err, "initialized failed")
	}
	c.Server = server
	c.initializeResult = &result
	return nil
}

func (c *Client) InitializeResult(context.Context, *protocol.TextDocumentIdentifier) (*protocol.InitializeResult, error) {
	return c.initializeResult, nil
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

func DidOpen(ctx context.Context, server protocol.Server, filename string, body []byte) error {
	return server.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        text.ToURI(filename),
			LanguageID: fileLanguage(filename),
			Version:    0,
			Text:       string(body),
		},
	})
}

func DidClose(ctx context.Context, server protocol.Server, filename string) error {
	return server.DidClose(ctx, &protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: text.ToURI(filename),
		},
	})
}

func DidSave(ctx context.Context, server protocol.Server, filename string) error {
	return server.DidSave(ctx, &protocol.DidSaveTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{
				URI: text.ToURI(filename),
			},
			// TODO(fhs): add text field for includeText option
		},
	})
}

func DidChange(ctx context.Context, server protocol.Server, filename string, body []byte) error {
	return server.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
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

// SendMessage exists only to implement proxy.Server.
func (c *Client) Version(context.Context) (int, error) {
	panic("intentionally not implemented")
}

// SendMessage exists only to implement proxy.Server.
func (c *Client) SendMessage(context.Context, *proxy.Message) error {
	panic("intentionally not implemented")
}

// SendMessage exists only to implement proxy.Server.
func (c *Client) WorkspaceFolders(context.Context) ([]protocol.WorkspaceFolder, error) {
	panic("intentionally not implemented")
}

func ServerProvidesCodeAction(cap *protocol.ServerCapabilities, kind protocol.CodeActionKind) bool {
	switch ap := cap.CodeActionProvider.(type) {
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

func DirsToWorkspaceFolders(dirs []string) ([]protocol.WorkspaceFolder, error) {
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
