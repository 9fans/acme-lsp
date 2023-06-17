// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package proxy

import (
	"context"
	"log"

	"github.com/fhs/go-lsp-internal/lsp/protocol"
	"github.com/sourcegraph/jsonrpc2"
)

type DocumentUri = string

type clientHandler struct {
	client    Client
	lspClient jsonrpc2.Handler
}

func (h *clientHandler) Handle(ctx context.Context, conn *jsonrpc2.Conn, r *jsonrpc2.Request) {
	ok, err := clientDispatch(ctx, h.client, conn, r)
	if !ok {
		h.lspClient.Handle(ctx, conn, r)
		return
	}
	if err != nil {
		log.Printf("rpc reply failed: %v", err)
	}
}

func NewClientHandler(client Client) jsonrpc2.Handler {
	return &clientHandler{
		client:    client,
		lspClient: protocol.NewClientHandler(client),
	}
}

type serverHandler struct {
	server    Server
	lspServer jsonrpc2.Handler
}

func (h *serverHandler) Handle(ctx context.Context, conn *jsonrpc2.Conn, r *jsonrpc2.Request) {
	ok, err := serverDispatch(ctx, h.server, conn, r)
	if !ok {
		h.lspServer.Handle(ctx, conn, r)
		return
	}
	if err != nil {
		log.Printf("rpc reply failed: %v", err)
	}
}

func NewServerHandler(server Server) jsonrpc2.Handler {
	return &serverHandler{
		server:    server,
		lspServer: protocol.NewServerHandler(server),
	}
}

func NewClient(conn *jsonrpc2.Conn) Client {
	return &clientDispatcher{
		Conn:   conn,
		Client: protocol.NewClient(conn),
	}
}

func NewServer(conn *jsonrpc2.Conn) Server {
	return &serverDispatcher{
		Conn:   conn,
		Server: protocol.NewServer(conn),
	}
}

func reply(ctx context.Context, conn *jsonrpc2.Conn, id jsonrpc2.ID, result interface{}, err error) error {
	if err != nil {
		rpcerr := &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: err.Error(),
		}
		return conn.ReplyWithError(ctx, id, rpcerr)
	}
	return conn.Reply(ctx, id, result)
}

func sendParseError(ctx context.Context, conn *jsonrpc2.Conn, id jsonrpc2.ID, err error) error {
	rpcerr := &jsonrpc2.Error{
		Code:    jsonrpc2.CodeParseError,
		Message: err.Error(),
	}
	return conn.ReplyWithError(ctx, id, rpcerr)
}

type ExecuteCommandOnDocumentParams struct {
	TextDocument         protocol.TextDocumentIdentifier
	ExecuteCommandParams protocol.ExecuteCommandParams
}
