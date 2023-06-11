package proxy

import (
	"context"
	"fmt"
	"log"
)

const Debug = false

type Client interface{}

type clientDispatcher struct {
	*jsonrpc2.Conn
	protocol.Client
}

type lspClientDispatcher struct {
	Client
}

func (c *lspClientDispatcher) ShowMessage(context.Context, *protocol.ShowMessageParams) error {
	return fmt.Errorf("ShowMessage not implemented")
}

func (c *lspClientDispatcher) LogMessage(ctx context.Context, params *protocol.LogMessageParams) error {
	if Debug {
		log.Printf("log: proxy %v: %v\n", params.Type, params.Message)
	}
	return nil
}

func (c *lspClientDispatcher) Event(context.Context, *interface{}) error {
	return fmt.Errorf("Event not implemented")
}

func (c *lspClientDispatcher) PublishDiagnostics(context.Context, *protocol.PublishDiagnosticsParams) error {
	return fmt.Errorf("PublishDiagnostics not implemented")
}

func (c *lspClientDispatcher) WorkspaceFolders(context.Context) ([]protocol.WorkspaceFolder, error) {
	return nil, fmt.Errorf("WorkspaceFolders not implemented")
}

func (c *lspClientDispatcher) Configuration(context.Context, *protocol.ParamConfig) ([]interface{}, error) {
	return nil, fmt.Errorf("Configuration not implemented")
}

func (c *lspClientDispatcher) RegisterCapability(context.Context, *protocol.RegistrationParams) error {
	return fmt.Errorf("RegisterCapability not implemented")
}

func (c *lspClientDispatcher) UnregisterCapability(context.Context, *protocol.UnregistrationParams) error {
	return fmt.Errorf("UnregisterCapability not implemented")
}

func (c *lspClientDispatcher) ShowMessageRequest(context.Context, *protocol.ShowMessageRequestParams) (*protocol.MessageActionItem, error) {
	return nil, fmt.Errorf("ShowMessageRequest not implemented")
}

func (c *lspClientDispatcher) ApplyEdit(context.Context, *protocol.ApplyWorkspaceEditParams) (*protocol.ApplyWorkspaceEditResponse, error) {
	return nil, fmt.Errorf("ApplyEdit not implemented")
}
