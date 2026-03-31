package acmelsp

import (
	"context"
	"fmt"
	"log"
	"net"
	"path/filepath"
	"sync"

	"9fans.net/internal/go-lsp/lsp/protocol"
	"github.com/sourcegraph/jsonrpc2"

	"9fans.net/acme-lsp/internal/lsp"
	"9fans.net/acme-lsp/internal/lsp/acmelsp/config"
	"9fans.net/acme-lsp/internal/lsp/proxy"
	"9fans.net/acme-lsp/internal/lsp/text"
)

var Verbose = false

type DiagnosticsWriter interface {
	WriteDiagnostics(params *protocol.PublishDiagnosticsParams)
}

// clientHandler handles JSON-RPC requests and notifications.
type clientHandler struct {
	cfg        *ClientConfig
	hideDiag   bool
	diagWriter DiagnosticsWriter
	diag       map[protocol.DocumentURI][]protocol.Diagnostic
	mu         sync.Mutex
	proxy.NotImplementedClient
}

func (h *clientHandler) ShowMessage(ctx context.Context, params *protocol.ShowMessageParams) error {
	if h.cfg != nil && h.cfg.FilenameHandler != nil {
		log.Printf("LSP %s %v: %v\n", h.cfg.FilenameHandler.ServerKey, params.Type, params.Message)
	} else {
		log.Printf("LSP %v: %v\n", params.Type, params.Message)
	}
	return nil
}

func (h *clientHandler) LogMessage(ctx context.Context, params *protocol.LogMessageParams) error {
	if h.cfg.Logger != nil {
		h.cfg.Logger.Printf("%v: %v\n", params.Type, params.Message)
		return nil
	}
	if params.Type == protocol.Error || params.Type == protocol.Warning || Verbose {
		log.Printf("log: LSP %v: %v\n", params.Type, params.Message)
	}
	return nil
}

func (h *clientHandler) Event(context.Context, *interface{}) error {
	return nil
}

func (h *clientHandler) PublishDiagnostics(ctx context.Context, params *protocol.PublishDiagnosticsParams) error {
	if h.hideDiag {
		return nil
	}

	h.diagWriter.WriteDiagnostics(params)
	return nil
}

func (h *clientHandler) WorkspaceFolders(context.Context) ([]protocol.WorkspaceFolder, error) {
	return nil, nil
}

func (h *clientHandler) Configuration(context.Context, *protocol.ParamConfiguration) ([]interface{}, error) {
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

func (h *clientHandler) ApplyEdit(ctx context.Context, params *protocol.ApplyWorkspaceEditParams) (*protocol.ApplyWorkspaceEditResult, error) {
	err := editWorkspace(&params.Edit, &text.AcmeMenu{})
	if err != nil {
		return &protocol.ApplyWorkspaceEditResult{Applied: false, FailureReason: err.Error()}, nil
	}
	return &protocol.ApplyWorkspaceEditResult{Applied: true}, nil
}

// ClientConfig contains LSP client configuration values.
type ClientConfig struct {
	*config.Server
	*config.FilenameHandler
	RootDirectory string                     // used to compute RootURI in initialization
	HideDiag      bool                       // don't write diagnostics to DiagWriter
	RPCTrace      bool                       // print LSP rpc trace to stderr
	DiagWriter    DiagnosticsWriter          // notification handler writes diagnostics here
	Workspaces    []protocol.WorkspaceFolder // initial workspace folders
	Logger        *log.Logger
}

// docState holds the tracked state of an open document.
type docState struct {
	version int32
	end     protocol.Position // end position of the last synced content
}

// Client represents a LSP client connection.
type Client struct {
	protocol.Server
	initializeResult *protocol.InitializeResult
	cfg              *ClientConfig
	rpc              *jsonrpc2.Conn
	openDocs         map[protocol.DocumentURI]docState
	mu               sync.Mutex
}

func NewClient(conn net.Conn, cfg *ClientConfig) (*Client, error) {
	c := &Client{
		cfg:      cfg,
		openDocs: make(map[protocol.DocumentURI]docState),
	}
	if err := c.init(conn, cfg); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Client) Close() error {
	return c.rpc.Close()
}

func (c *Client) init(conn net.Conn, cfg *ClientConfig) error {
	ctx := context.Background()
	stream := jsonrpc2.NewBufferedStream(conn, jsonrpc2.VSCodeObjectCodec{})
	handler := proxy.NewClientHandler(&clientHandler{
		cfg:        cfg,
		hideDiag:   cfg.HideDiag,
		diagWriter: cfg.DiagWriter,
		diag:       make(map[protocol.DocumentURI][]protocol.Diagnostic),
	})
	var opts []jsonrpc2.ConnOpt
	if cfg.RPCTrace {
		opts = append(opts, lsp.LogMessages(log.Default()))
	}
	if c.rpc != nil {
		c.rpc.Close()
	}
	rpc := jsonrpc2.NewConn(ctx, stream, handler, opts...)
	server := protocol.NewServer(rpc)
	go func() {
		<-rpc.DisconnectNotify()
		log.Printf("jsonrpc2 client connection to LSP sever disconnected\n")
	}()
	c.rpc = rpc

	d, err := filepath.Abs(cfg.RootDirectory)
	if err != nil {
		return err
	}
	params := &protocol.ParamInitialize{
		XInitializeParams: protocol.XInitializeParams{
			RootURI: text.ToURI(d),
			Capabilities: protocol.ClientCapabilities{
				TextDocument: protocol.TextDocumentClientCapabilities{
					CodeAction: protocol.CodeActionClientCapabilities{
						CodeActionLiteralSupport: protocol.ClientCodeActionLiteralOptions{
							CodeActionKind: protocol.ClientCodeActionKindOptions{
								ValueSet: []protocol.CodeActionKind{
									protocol.SourceOrganizeImports,
								},
							},
						},
					},
					DocumentSymbol: protocol.DocumentSymbolClientCapabilities{
						HierarchicalDocumentSymbolSupport: true,
					},
					Completion: protocol.CompletionClientCapabilities{
						CompletionItem: protocol.ClientCompletionItemOptions{
							TagSupport: &protocol.CompletionItemTagOptions{
								ValueSet: []protocol.CompletionItemTag{},
							},
						},
					},
					SemanticTokens: protocol.SemanticTokensClientCapabilities{
						Formats:        []protocol.TokenFormat{},
						TokenModifiers: []string{},
						TokenTypes:     []string{},
					},
				},
				Workspace: protocol.WorkspaceClientCapabilities{
					WorkspaceFolders: true,
					ApplyEdit:        true,
				},
			},
			InitializationOptions: cfg.Options,
		},
		WorkspaceFoldersInitializeParams: protocol.WorkspaceFoldersInitializeParams{
			WorkspaceFolders: cfg.Workspaces,
		},
	}

	result, err := server.Initialize(ctx, params)
	if err != nil {
		return fmt.Errorf("initialize failed: %v", err)
	}
	if err := server.Initialized(ctx, &protocol.InitializedParams{}); err != nil {
		return fmt.Errorf("initialized failed: %v", err)
	}
	c.Server = server
	c.initializeResult = result
	return nil
}

// InitializeResult implements proxy.Server.
func (c *Client) InitializeResult(context.Context, *protocol.TextDocumentIdentifier) (*protocol.InitializeResult, error) {
	return c.initializeResult, nil
}

// Version exists only to implement proxy.Server.
func (c *Client) Version(context.Context) (int, error) {
	panic("intentionally not implemented")
}

// WorkspaceFolders exists only to implement proxy.Server.
func (c *Client) WorkspaceFolders(context.Context) ([]protocol.WorkspaceFolder, error) {
	panic("intentionally not implemented")
}

// ExecuteCommandOnDocument implements proxy.Server.
func (s *Client) ExecuteCommandOnDocument(ctx context.Context, params *proxy.ExecuteCommandOnDocumentParams) (interface{}, error) {
	return s.Server.ExecuteCommand(ctx, &params.ExecuteCommandParams)
}

func (s *Client) didOpen(ctx context.Context, params *proxy.SyncDocumentParams) error {
	var langID protocol.LanguageKind
	if s.cfg != nil && s.cfg.FilenameHandler != nil {
		langID = s.cfg.FilenameHandler.LanguageID
	}
	if langID == "" {
		langID = lsp.DetectLanguage(text.ToPath(params.TextDocument.URI))
	}
	return s.Server.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        params.TextDocument.URI,
			LanguageID: langID,
			Version:    0,
			Text:       params.Content,
		},
	})
}

func (s *Client) SyncDocument(ctx context.Context, params *proxy.SyncDocumentParams) error {
	line, col := text.GetLastPosition(params.Content)
	newEnd := protocol.Position{
		Line:      uint32(line),
		Character: uint32(col),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.openDocs[params.TextDocument.URI]
	if !ok {
		// Document is not open, so open it
		if err := s.didOpen(ctx, params); err != nil {
			return err
		}
		s.openDocs[params.TextDocument.URI] = docState{version: 1, end: newEnd}
		return nil
	}

	// Document was previously opened, so update the content.
	// Only include Range if the server supports incremental sync.
	// The range covers the entire old document content.
	var rg *protocol.Range
	if s.initializeResult != nil && lsp.ServerSupportsIncrementalSync(&s.initializeResult.Capabilities) {
		rg = &protocol.Range{
			Start: protocol.Position{Line: 0, Character: 0},
			End:   state.end,
		}
	}
	newVersion := state.version + 1
	err := s.Server.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: params.TextDocument,
			Version:                newVersion,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			{
				Range: rg,
				Text:  params.Content,
			},
		},
	})
	if err != nil {
		return err
	}
	s.openDocs[params.TextDocument.URI] = docState{version: newVersion, end: newEnd}
	return nil
}

func (s *Client) DidClose(ctx context.Context, params *protocol.DidCloseTextDocumentParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.openDocs[params.TextDocument.URI]; !ok {
		return nil // document is not open
	}
	if err := s.Server.DidClose(ctx, params); err != nil {
		return err
	}
	delete(s.openDocs, params.TextDocument.URI)
	return nil
}
func (s *Client) DidChange(context.Context, *protocol.DidChangeTextDocumentParams) error {
	return fmt.Errorf("not implemented -- use SyncDocument")
}

func (s *Client) DidOpen(context.Context, *protocol.DidOpenTextDocumentParams) error {
	return fmt.Errorf("not implemented -- use SyncDocument")
}
