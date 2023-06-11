// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package proxy

import (
	"context"
)

type DocumentUri = string

type canceller struct{ jsonrpc2.EmptyHandler }

type clientHandler struct {
	canceller
	client Client
}

type serverHandler struct {
	canceller
	server Server
}

func (canceller) Cancel(ctx context.Context, conn *jsonrpc2.Conn, id jsonrpc2.ID, cancelled bool) bool {
	if cancelled {
		return false
	}
	ctx = xcontext.Detach(ctx)
	ctx, done := trace.StartSpan(ctx, "protocol.canceller")
	defer done()
	conn.Notify(ctx, "$/cancelRequest", &CancelParams{ID: id})
	return true
}

func NewClient(ctx context.Context, stream jsonrpc2.Stream, client Client) (context.Context, *jsonrpc2.Conn, Server) {
	c := &lspClientDispatcher{
		Client: client,
	}
	ctx, conn, server := protocol.NewClient(ctx, stream, c)
	conn.AddHandler(&clientHandler{client: client})
	s := &serverDispatcher{
		Conn:   conn,
		Server: server,
	}
	return ctx, conn, s
}

func NewServer(ctx context.Context, stream jsonrpc2.Stream, server Server) (context.Context, *jsonrpc2.Conn, Client) {
	s := &lspServerDispatcher{
		Server: server,
	}
	ctx, conn, client := protocol.NewServer(ctx, stream, s)
	conn.AddHandler(&serverHandler{server: server})
	c := &clientDispatcher{
		Conn:   conn,
		Client: client,
	}
	return ctx, conn, c
}

func sendParseError(ctx context.Context, req *jsonrpc2.Request, err error) {
	if _, ok := err.(*jsonrpc2.Error); !ok {
		err = jsonrpc2.NewErrorf(jsonrpc2.CodeParseError, "%v", err)
	}
	if err := req.Reply(ctx, nil, err); err != nil {
		log.Error(ctx, "", err)
	}
}

type ExecuteCommandOnDocumentParams struct {
	TextDocument         protocol.TextDocumentIdentifier
	ExecuteCommandParams protocol.ExecuteCommandParams
}
