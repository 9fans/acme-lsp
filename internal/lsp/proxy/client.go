package proxy

import (
	"context"
	"fmt"

	"github.com/fhs/acme-lsp/internal/golang_org_x_tools/jsonrpc2"
	"github.com/fhs/acme-lsp/internal/lsp/protocol"
)

type Client interface{}

type clientDispatcher struct {
	*jsonrpc2.Conn
	protocol.Client
}

type lspClientDispatcher struct {
	Client
}

func (c *lspClientDispatcher) ShowMessage(context.Context, *protocol.ShowMessageParams) error {
	return fmt.Errorf("not implemented")
}

func (c *lspClientDispatcher) LogMessage(context.Context, *protocol.LogMessageParams) error {
	return fmt.Errorf("not implemented")
}

func (c *lspClientDispatcher) Event(context.Context, *interface{}) error {
	return fmt.Errorf("not implemented")
}

func (c *lspClientDispatcher) PublishDiagnostics(context.Context, *protocol.PublishDiagnosticsParams) error {
	return fmt.Errorf("not implemented")
}

func (c *lspClientDispatcher) WorkspaceFolders(context.Context) ([]protocol.WorkspaceFolder, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *lspClientDispatcher) Configuration(context.Context, *protocol.ParamConfig) ([]interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *lspClientDispatcher) RegisterCapability(context.Context, *protocol.RegistrationParams) error {
	return fmt.Errorf("not implemented")
}

func (c *lspClientDispatcher) UnregisterCapability(context.Context, *protocol.UnregistrationParams) error {
	return fmt.Errorf("not implemented")
}

func (c *lspClientDispatcher) ShowMessageRequest(context.Context, *protocol.ShowMessageRequestParams) (*protocol.MessageActionItem, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *lspClientDispatcher) ApplyEdit(context.Context, *protocol.ApplyWorkspaceEditParams) (*protocol.ApplyWorkspaceEditResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
