package proxy

import "github.com/fhs/acme-lsp/internal/golang_org_x_tools/jsonrpc2"

type Client interface{}

type clientDispatcher struct {
	*jsonrpc2.Conn
}
