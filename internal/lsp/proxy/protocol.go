// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package proxy

import (
	"context"
	"log"

	"9fans.net/internal/go-lsp/lsp/protocol"
	"github.com/sourcegraph/jsonrpc2"
)

type DocumentUri = string

type clientHandler struct {
	client Client
}

func (h *clientHandler) Handle(ctx context.Context, conn *jsonrpc2.Conn, r *jsonrpc2.Request) {
	if Debug {
		log.Printf("proxy: client handler %v\n", r.Method)
	}
	ok, err := clientDispatch(ctx, h.client, conn, r)
	if !ok {
		ok, err = protocol.ClientDispatch(ctx, h.client, conn, r)
	}
	if !ok {
		rpcerr := &jsonrpc2.Error{
			Code:    jsonrpc2.CodeMethodNotFound,
			Message: "method not implemented",
		}
		err = conn.Reply(ctx, r.ID, rpcerr)
	}
	if err != nil {
		log.Printf("proxy: client rpc reply failed for %v: %v", r.Method, err)
	}
}

func NewClientHandler(client Client) jsonrpc2.Handler {
	return &clientHandler{
		client: client,
	}
}

type serverHandler struct {
	server Server
}

func (h *serverHandler) Handle(ctx context.Context, conn *jsonrpc2.Conn, r *jsonrpc2.Request) {
	if Debug {
		log.Printf("proxy: server handler %v\n", r.Method)
	}
	ok, err := serverDispatch(ctx, h.server, conn, r)
	if !ok {
		ok, err = protocol.ServerDispatch(ctx, h.server, conn, r)
	}
	if !ok {
		rpcerr := &jsonrpc2.Error{
			Code:    jsonrpc2.CodeMethodNotFound,
			Message: "method not implemented",
		}
		err = conn.Reply(ctx, r.ID, rpcerr)
	}
	if err != nil {
		log.Printf("proxy: server rpc reply failed for %v: %v", r.Method, err)
	}
}

func NewServerHandler(server Server) jsonrpc2.Handler {
	return &serverHandler{
		server: server,
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
