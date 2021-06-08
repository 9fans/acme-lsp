// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package proxy

import (
	"context"
	"net"

	"github.com/fhs/acme-lsp/internal/golang_org_x_tools/jsonrpc2"
	"github.com/fhs/acme-lsp/internal/golang_org_x_tools/lsp/protocol"
	"github.com/fhs/acme-lsp/internal/golang_org_x_tools/telemetry/log"
	"github.com/fhs/acme-lsp/internal/golang_org_x_tools/telemetry/trace"
	"github.com/fhs/acme-lsp/internal/golang_org_x_tools/xcontext"
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

func ServerDispatcher(conn *jsonrpc2.Conn, server protocol.Server) Server {
	return &serverDispatcher{
		Conn:   conn,
		Server: server,
	}
}

func ClientDispatcher(conn *jsonrpc2.Conn, client protocol.Client) Client {
	return &clientDispatcher{
		Conn:   conn,
		Client: client,
	}
}

func NewClient(ctx context.Context, conn net.Conn, client Client) Server {
	stream := jsonrpc2.NewHeaderStream(conn, conn)
	cc := jsonrpc2.NewConn(stream)
	server := ServerDispatcher(cc, protocol.ServerDispatcher(cc))
	cc.AddHandler(protocol.ClientHandler(&lspClientDispatcher{Client: client}))
	cc.AddHandler(protocol.Canceller{})
	cc.AddHandler(&clientHandler{client: client})
	go cc.Run(ctx)
	return server
}

type streamServer struct {
	server Server
}

func NewStreamServer(server Server) jsonrpc2.StreamServer {
	return &streamServer{
		server: server,
	}
}

func (s *streamServer) ServeStream(ctx context.Context, stream jsonrpc2.Stream) error {
	cc := jsonrpc2.NewConn(stream)
	// The proxy server doesn't need to make calls to the client.
	//client := ClientDispatcher(cc, protocol.ClientDispatcher(cc))
	cc.AddHandler(protocol.ServerHandler(&lspServerDispatcher{Server: s.server}))
	cc.AddHandler(protocol.Canceller{})
	cc.AddHandler(&serverHandler{server: s.server})
	return cc.Run(ctx)
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
