package proxy

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fhs/acme-lsp/internal/golang_org_x_tools/jsonrpc2"
	"github.com/fhs/acme-lsp/internal/golang_org_x_tools/telemetry/log"
	"github.com/fhs/acme-lsp/internal/lsp/protocol"
)

// Version is used to detect if acme-lsp and L are speaking the same protocol.
const Version = 1

// Server implements a subset of an LSP protocol server as defined by protocol.Server and
// some custom acme-lsp specific methods.
type subLspServer interface {
	// Version returns the protocol version.
	Version(context.Context) (int, error)

	// WorkspaceFolders returns the workspace folders currently being managed by acme-lsp.
	// In LSP, this method is implemented by the client, but in our case acme-lsp is managing
	// the workspace folders, so this has to be implemented by the acme-lsp proxy server.
	WorkspaceFolders(context.Context) ([]protocol.WorkspaceFolder, error)

	// InitializeResult returns the initialize response from the LSP server.
	// This is useful for L command to get initialization results (e.g. server capabilities)
	// of an already initialized LSP server.
	InitializeResult(context.Context, *protocol.TextDocumentIdentifier) (*protocol.InitializeResult, error)

	// ExecuteCommandOnDocument is the same as ExecuteCommand, but
	// params contain the TextDocumentIdentifier of the original
	// CodeAction so that the server implemention can multiplex
	// ExecuteCommand request to the right server.
	ExecuteCommandOnDocument(context.Context, *ExecuteCommandOnDocumentParams) (interface{}, error)

	DidChange(context.Context, *protocol.DidChangeTextDocumentParams) error
	DidChangeWorkspaceFolders(context.Context, *protocol.DidChangeWorkspaceFoldersParams) error
	Completion(context.Context, *protocol.CompletionParams) (*protocol.CompletionList, error)
	Definition(context.Context, *protocol.DefinitionParams) ([]protocol.Location, error)
	Formatting(context.Context, *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error)
	CodeAction(context.Context, *protocol.CodeActionParams) ([]protocol.CodeAction, error)
	Hover(context.Context, *protocol.HoverParams) (*protocol.Hover, error)
	Implementation(context.Context, *protocol.ImplementationParams) ([]protocol.Location, error)
	References(context.Context, *protocol.ReferenceParams) ([]protocol.Location, error)
	Rename(context.Context, *protocol.RenameParams) (*protocol.WorkspaceEdit, error)
	SignatureHelp(context.Context, *protocol.SignatureHelpParams) (*protocol.SignatureHelp, error)
	DocumentSymbol(context.Context, *protocol.DocumentSymbolParams) ([]protocol.DocumentSymbol, error)
	TypeDefinition(context.Context, *protocol.TypeDefinitionParams) ([]protocol.Location, error)
}

type ExtendServer interface {
	Metadata(context.Context, *protocol.MetadataParams) (*protocol.MetaSourceRsponse, error)
}

type Server interface {
	subLspServer
	ExtendServer
}

func (h serverHandler) Deliver(ctx context.Context, r *jsonrpc2.Request, delivered bool) bool {
	if delivered {
		return false
	}
	switch r.Method {
	case "$/cancelRequest":
		var params CancelParams
		if err := json.Unmarshal(*r.Params, &params); err != nil {
			sendParseError(ctx, r, err)
			return true
		}
		r.Conn().Cancel(params.ID)
		return true

	case "acme-lsp/version": // req
		resp, err := h.server.Version(ctx)
		if err := r.Reply(ctx, resp, err); err != nil {
			log.Error(ctx, "", err)
		}
		return true

	case "acme-lsp/workspaceFolders": // req
		resp, err := h.server.WorkspaceFolders(ctx)
		if err := r.Reply(ctx, resp, err); err != nil {
			log.Error(ctx, "", err)
		}
		return true

	case "acme-lsp/initializeResult": // req
		var params protocol.TextDocumentIdentifier
		if err := json.Unmarshal(*r.Params, &params); err != nil {
			sendParseError(ctx, r, err)
			return true
		}
		resp, err := h.server.InitializeResult(ctx, &params)
		if err := r.Reply(ctx, resp, err); err != nil {
			log.Error(ctx, "", err)
		}
		return true

	case "acme-lsp/executeCommandOnDocument": // req
		var params ExecuteCommandOnDocumentParams
		if err := json.Unmarshal(*r.Params, &params); err != nil {
			sendParseError(ctx, r, err)
			return true
		}
		resp, err := h.server.ExecuteCommandOnDocument(ctx, &params)
		if err := r.Reply(ctx, resp, err); err != nil {
			log.Error(ctx, "", err)
		}
		return true
	case protocol.MetadataEndpoint: // req csharp/metadata
		var params protocol.MetadataParams
		if err := json.Unmarshal(*r.Params, &params); err != nil {
			sendParseError(ctx, r, err)
			return true
		}

		resp, err := h.server.Metadata(ctx, &params)
		if err := r.Reply(ctx, resp, err); err != nil {
			log.Error(ctx, "", err)
		}
		return true
	default:
		return false
	}
}

type serverDispatcher struct {
	*jsonrpc2.Conn
	protocol.Server
}

func (s *serverDispatcher) Version(ctx context.Context) (int, error) {
	var result int
	if err := s.Conn.Call(ctx, "acme-lsp/version", nil, &result); err != nil {
		return 0, err
	}
	return result, nil
}

func (s *serverDispatcher) WorkspaceFolders(ctx context.Context) ([]protocol.WorkspaceFolder, error) {
	var result []protocol.WorkspaceFolder
	if err := s.Conn.Call(ctx, "acme-lsp/workspaceFolders", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *serverDispatcher) InitializeResult(ctx context.Context, params *protocol.TextDocumentIdentifier) (*protocol.InitializeResult, error) {
	var result protocol.InitializeResult
	if err := s.Conn.Call(ctx, "acme-lsp/initializeResult", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *serverDispatcher) ExecuteCommandOnDocument(ctx context.Context, params *ExecuteCommandOnDocumentParams) (interface{}, error) {
	var result interface{}
	if err := s.Conn.Call(ctx, "acme-lsp/executeCommandOnDocument", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

type CancelParams struct {
	/**
	 * The request id to cancel.
	 */
	ID jsonrpc2.ID `json:"id"`
}

type Message struct {
	Data string
	Attr map[string]string
}

type lspServerDispatcher struct {
	Server
}

func (s *lspServerDispatcher) Initialized(context.Context, *protocol.InitializedParams) error {
	return fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) Metadata(context.Context, *protocol.MetadataParams) (*protocol.MetaSourceRsponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) Exit(context.Context) error {
	return fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) DidChangeConfiguration(context.Context, *protocol.DidChangeConfigurationParams) error {
	return fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) DidOpen(context.Context, *protocol.DidOpenTextDocumentParams) error {
	return fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) DidClose(context.Context, *protocol.DidCloseTextDocumentParams) error {
	return fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) DidSave(context.Context, *protocol.DidSaveTextDocumentParams) error {
	return fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) WillSave(context.Context, *protocol.WillSaveTextDocumentParams) error {
	return fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) DidChangeWatchedFiles(context.Context, *protocol.DidChangeWatchedFilesParams) error {
	return fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) Progress(context.Context, *protocol.ProgressParams) error {
	return fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) SetTraceNotification(context.Context, *protocol.SetTraceParams) error {
	return fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) LogTraceNotification(context.Context, *protocol.LogTraceParams) error {
	return fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) DocumentColor(context.Context, *protocol.DocumentColorParams) ([]protocol.ColorInformation, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) ColorPresentation(context.Context, *protocol.ColorPresentationParams) ([]protocol.ColorPresentation, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) FoldingRange(context.Context, *protocol.FoldingRangeParams) ([]protocol.FoldingRange, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) Declaration(context.Context, *protocol.DeclarationParams) ([]protocol.DeclarationLink, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) SelectionRange(context.Context, *protocol.SelectionRangeParams) ([]protocol.SelectionRange, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) Initialize(context.Context, *protocol.ParamInitia) (*protocol.InitializeResult, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) Shutdown(context.Context) error {
	return fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) WillSaveWaitUntil(context.Context, *protocol.WillSaveTextDocumentParams) ([]protocol.TextEdit, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) Resolve(context.Context, *protocol.CompletionItem) (*protocol.CompletionItem, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) DocumentHighlight(context.Context, *protocol.DocumentHighlightParams) ([]protocol.DocumentHighlight, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) Symbol(context.Context, *protocol.WorkspaceSymbolParams) ([]protocol.SymbolInformation, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) CodeLens(context.Context, *protocol.CodeLensParams) ([]protocol.CodeLens, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) ResolveCodeLens(context.Context, *protocol.CodeLens) (*protocol.CodeLens, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) RangeFormatting(context.Context, *protocol.DocumentRangeFormattingParams) ([]protocol.TextEdit, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) OnTypeFormatting(context.Context, *protocol.DocumentOnTypeFormattingParams) ([]protocol.TextEdit, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) PrepareRename(context.Context, *protocol.PrepareRenameParams) (*protocol.Range, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) DocumentLink(context.Context, *protocol.DocumentLinkParams) ([]protocol.DocumentLink, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) ResolveDocumentLink(context.Context, *protocol.DocumentLink) (*protocol.DocumentLink, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) ExecuteCommand(context.Context, *protocol.ExecuteCommandParams) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}
