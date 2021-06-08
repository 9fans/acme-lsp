package acmelsp

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/fhs/acme-lsp/internal/golang_org_x_tools/jsonrpc2"
	"github.com/fhs/acme-lsp/internal/golang_org_x_tools/lsp/protocol"
	"github.com/fhs/acme-lsp/internal/lsp/acmelsp/config"
	"github.com/fhs/acme-lsp/internal/lsp/proxy"
	"github.com/fhs/acme-lsp/internal/lsp/text"
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
}

func (h *clientHandler) ShowMessage(ctx context.Context, params *protocol.ShowMessageParams) error {
	log.Printf("LSP %v: %v\n", params.Type, params.Message)
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

func (h *clientHandler) ApplyEdit(ctx context.Context, params *protocol.ApplyWorkspaceEditParams) (*protocol.ApplyWorkspaceEditResponse, error) {
	err := editWorkspace(&params.Edit)
	if err != nil {
		return &protocol.ApplyWorkspaceEditResponse{Applied: false, FailureReason: err.Error()}, nil
	}
	return &protocol.ApplyWorkspaceEditResponse{Applied: true}, nil
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

// Client represents a LSP client connection.
type Client struct {
	protocol.Server
	initializeResult *protocol.InitializeResult
	cfg              *ClientConfig
}

func NewClient(conn net.Conn, cfg *ClientConfig) (*Client, error) {
	c := &Client{cfg: cfg}
	if err := c.init(conn, cfg); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Client) init(conn net.Conn, cfg *ClientConfig) error {
	ctx := context.Background()
	stream := jsonrpc2.NewHeaderStream(conn, conn)
	if cfg.RPCTrace {
		stream = protocol.LoggingStream(stream, os.Stderr)
	}
	cc := jsonrpc2.NewConn(stream)
	server := protocol.ServerDispatcher(cc)
	client := &clientHandler{
		cfg:        cfg,
		hideDiag:   cfg.HideDiag,
		diagWriter: cfg.DiagWriter,
		diag:       make(map[protocol.DocumentURI][]protocol.Diagnostic),
	}
	cc.AddHandler(protocol.ClientHandler(client))
	cc.AddHandler(protocol.Canceller{})
	ctx = protocol.WithClient(ctx, client)
	go cc.Run(ctx)

	d, err := filepath.Abs(cfg.RootDirectory)
	if err != nil {
		return err
	}
	params := &protocol.ParamInitialize{
		InitializeParams: protocol.InitializeParams{
			InnerInitializeParams: protocol.InnerInitializeParams{
				RootURI: text.ToURI(d),
				Capabilities: protocol.ClientCapabilities{
					// Workspace: ..., (struct literal)
					TextDocument: protocol.TextDocumentClientCapabilities{
						CodeAction: protocol.CodeActionClientCapabilities{
							CodeActionLiteralSupport: CodeActionLiteralSupport{
								// CodeActionKind: ..., (struct literal)
							},
						},
						DocumentSymbol: protocol.DocumentSymbolClientCapabilities{
							HierarchicalDocumentSymbolSupport: true,
						},
					},
				},
				InitializationOptions: cfg.Options,
			},
			WorkspaceFoldersInitializeParams: protocol.WorkspaceFoldersInitializeParams{
				WorkspaceFolders: cfg.Workspaces,
			},
		},
	}
	params.Capabilities.Workspace.WorkspaceFolders = true
	params.Capabilities.Workspace.ApplyEdit = true
	params.Capabilities.TextDocument.CodeAction.CodeActionLiteralSupport.CodeActionKind.ValueSet =
		[]protocol.CodeActionKind{protocol.SourceOrganizeImports}

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

// CodeActionLiteralSupport is a type alias that works around difficulty in initializing the pointer
// InitializeParams.Capabilities.TextDocument.CodeAction.CodeActionLiteralSupport.
type CodeActionLiteralSupport = struct {
	CodeActionKind struct {
		ValueSet []protocol.CodeActionKind `json:"valueSet"`
	} `json:"codeActionKind"`
}
