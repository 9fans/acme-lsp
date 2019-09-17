package proxy

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fhs/acme-lsp/internal/golang_org_x_tools/jsonrpc2"
	"github.com/fhs/acme-lsp/internal/golang_org_x_tools/telemetry/log"
	"github.com/fhs/acme-lsp/internal/lsp/protocol"
)

type Server interface {
	SendMessage(context.Context, *Message) error
	WorkspaceDirectories(context.Context) ([]string, error)
	AddWorkspaceDirectories(context.Context, []string) error
	RemoveWorkspaceDirectories(context.Context, []string) error
	Definition(context.Context, *protocol.DefinitionParams) ([]protocol.Location, error)
	References(context.Context, *protocol.ReferenceParams) ([]protocol.Location, error)
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

	case "acme-lsp/sendMessage": // notif
		var params Message
		if err := json.Unmarshal(*r.Params, &params); err != nil {
			sendParseError(ctx, r, err)
			return true
		}
		if err := h.server.SendMessage(ctx, &params); err != nil {
			log.Error(ctx, "", err)
		}
		return true

	case "acme-lsp/workspaceDirectories": // req
		resp, err := h.server.WorkspaceDirectories(ctx)
		if err := r.Reply(ctx, resp, err); err != nil {
			log.Error(ctx, "", err)
		}
		return true

	case "acme-lsp/addWorkspaceDirectories": // notif
		var params []string
		if err := json.Unmarshal(*r.Params, &params); err != nil {
			sendParseError(ctx, r, err)
			return true
		}
		if err := h.server.AddWorkspaceDirectories(ctx, params); err != nil {
			log.Error(ctx, "", err)
		}
		return true

	case "acme-lsp/removeWorkspaceDirectories": // notif
		var params []string
		if err := json.Unmarshal(*r.Params, &params); err != nil {
			sendParseError(ctx, r, err)
			return true
		}
		if err := h.server.RemoveWorkspaceDirectories(ctx, params); err != nil {
			log.Error(ctx, "", err)
		}
		return true

	case "textDocument/definition": // req
		var params protocol.DefinitionParams
		if err := json.Unmarshal(*r.Params, &params); err != nil {
			sendParseError(ctx, r, err)
			return true
		}
		resp, err := h.server.Definition(ctx, &params)
		if err := r.Reply(ctx, resp, err); err != nil {
			log.Error(ctx, "", err)
		}
		return true

	case "textDocument/references": // req
		var params protocol.ReferenceParams
		if err := json.Unmarshal(*r.Params, &params); err != nil {
			sendParseError(ctx, r, err)
			return true
		}
		resp, err := h.server.References(ctx, &params)
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

func (s *serverDispatcher) SendMessage(ctx context.Context, params *Message) error {
	return s.Conn.Notify(ctx, "acme-lsp/sendMessage", params)
}

func (s *serverDispatcher) WorkspaceDirectories(ctx context.Context) ([]string, error) {
	var result []string
	if err := s.Conn.Call(ctx, "acme-lsp/workspaceDirectories", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *serverDispatcher) AddWorkspaceDirectories(ctx context.Context, params []string) error {
	return s.Conn.Notify(ctx, "acme-lsp/addWorkspaceDirectories", &params)
}

func (s *serverDispatcher) RemoveWorkspaceDirectories(ctx context.Context, params []string) error {
	return s.Conn.Notify(ctx, "acme-lsp/removeWorkspaceDirectories", &params)
}

func (s *serverDispatcher) Definition(ctx context.Context, params *protocol.DefinitionParams) ([]protocol.Location, error) {
	var result []protocol.Location
	if err := s.Conn.Call(ctx, "textDocument/definition", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *serverDispatcher) References(ctx context.Context, params *protocol.ReferenceParams) ([]protocol.Location, error) {
	var result []protocol.Location
	if err := s.Conn.Call(ctx, "textDocument/references", params, &result); err != nil {
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

func (s *lspServerDispatcher) DidChangeWorkspaceFolders(context.Context, *protocol.DidChangeWorkspaceFoldersParams) error {
	return fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) Initialized(context.Context, *protocol.InitializedParams) error {
	return fmt.Errorf("not implemented")
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

func (s *lspServerDispatcher) DidChange(context.Context, *protocol.DidChangeTextDocumentParams) error {
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

func (s *lspServerDispatcher) Implementation(context.Context, *protocol.ImplementationParams) ([]protocol.Location, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) TypeDefinition(context.Context, *protocol.TypeDefinitionParams) ([]protocol.Location, error) {
	return nil, fmt.Errorf("not implemented")
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

func (s *lspServerDispatcher) Completion(context.Context, *protocol.CompletionParams) (*protocol.CompletionList, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) Resolve(context.Context, *protocol.CompletionItem) (*protocol.CompletionItem, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) Hover(context.Context, *protocol.HoverParams) (*protocol.Hover, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) SignatureHelp(context.Context, *protocol.SignatureHelpParams) (*protocol.SignatureHelp, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) Definition(context.Context, *protocol.DefinitionParams) ([]protocol.Location, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) References(context.Context, *protocol.ReferenceParams) ([]protocol.Location, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) DocumentHighlight(context.Context, *protocol.DocumentHighlightParams) ([]protocol.DocumentHighlight, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) DocumentSymbol(context.Context, *protocol.DocumentSymbolParams) ([]protocol.DocumentSymbol, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) CodeAction(context.Context, *protocol.CodeActionParams) ([]protocol.CodeAction, error) {
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

func (s *lspServerDispatcher) Formatting(context.Context, *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) RangeFormatting(context.Context, *protocol.DocumentRangeFormattingParams) ([]protocol.TextEdit, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) OnTypeFormatting(context.Context, *protocol.DocumentOnTypeFormattingParams) ([]protocol.TextEdit, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *lspServerDispatcher) Rename(context.Context, *protocol.RenameParams) (*protocol.WorkspaceEdit, error) {
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
