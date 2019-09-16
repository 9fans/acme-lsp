package proxy

import (
	"context"
	"encoding/json"

	"github.com/fhs/acme-lsp/internal/golang_org_x_tools/jsonrpc2"
	"github.com/fhs/acme-lsp/internal/golang_org_x_tools/telemetry/log"
)

type Server interface {
	SendMessage(context.Context, *Message) error
	WorkspaceDirectories(context.Context) ([]string, error)
	AddWorkspaceDirectories(context.Context, []string) error
	RemoveWorkspaceDirectories(context.Context, []string) error
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

	default:
		return false
	}
}

type serverDispatcher struct {
	*jsonrpc2.Conn
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
