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
	"github.com/fhs/acme-lsp/internal/lsp/acmelsp/config"
	"github.com/fhs/acme-lsp/internal/lsp/protocol"
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

func (h *clientHandler) Metadata(ctx context.Context, params *protocol.MetadataParams) error {
	log.Printf("LSP Metadata, params: %v\n", params)
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
	logger           *log.Logger
}

func NewClient(conn net.Conn, cfg *ClientConfig, logger *log.Logger) (*Client, error) {
	p := logger.Prefix()

	logger.SetPrefix(fmt.Sprintf("%s:client: ", p))
	c := &Client{cfg: cfg, logger: logger}
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
	ctx, rpc, serverDispatcher := protocol.NewClient(ctx, stream, &clientHandler{
		cfg:        cfg,
		hideDiag:   cfg.HideDiag,
		diagWriter: cfg.DiagWriter,
		diag:       make(map[protocol.DocumentURI][]protocol.Diagnostic),
	}, c.logger)
	go func() {
		err := rpc.Run(ctx)
		if err != nil {
			log.Printf("connection terminated: %v", err)
		}

		c.logger.Printf("client RCP runs  ")
	}()

	d, err := filepath.Abs(cfg.RootDirectory)
	if err != nil {
		return err
	}
	params := &protocol.InitializeParams{
		RootURI: text.ToURI(d),
		Capabilities: protocol.ClientCapabilities{
			// Workspace: ..., (struct literal)
			TextDocument: protocol.TextDocumentClientCapabilities{
				CodeAction: &protocol.CodeActionClientCapabilities{
					CodeActionLiteralSupport: &protocol.CodeActionLiteralSupport{
						// CodeActionKind: ..., (struct literal)
					},
				},
				DocumentSymbol: &protocol.DocumentSymbolClientCapabilities{
					HierarchicalDocumentSymbolSupport: true,
				},
			},
		},
		WorkspaceFolders:      cfg.Workspaces,
		InitializationOptions: cfg.Options,
	}
	params.Capabilities.Workspace.WorkspaceFolders = true
	params.Capabilities.Workspace.ApplyEdit = true
	params.Capabilities.TextDocument.CodeAction.CodeActionLiteralSupport.CodeActionKind.ValueSet =
		[]protocol.CodeActionKind{protocol.SourceOrganizeImports}

	var result protocol.InitializeResult
	if err := rpc.Call(ctx, "initialize", params, &result); err != nil {
		return fmt.Errorf("initialize failed: %v", err)
	}
	if err := rpc.Notify(ctx, "initialized", &protocol.InitializedParams{}); err != nil {
		return fmt.Errorf("initialized failed: %v", err)
	}
	c.Server = serverDispatcher
	c.initializeResult = &result
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
