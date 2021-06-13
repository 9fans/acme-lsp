// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package proxy

import (
	"context"
	"fmt"
	"net"

	"github.com/fhs/acme-lsp/internal/golang_org_x_tools/jsonrpc2"
	"github.com/fhs/acme-lsp/internal/golang_org_x_tools/lsp/protocol"
	"github.com/fhs/acme-lsp/internal/golang_org_x_tools/xcontext"
)

type DocumentUri = string

func ClientHandler(client Client, handler jsonrpc2.Handler) jsonrpc2.Handler {
	return func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		if ctx.Err() != nil {
			ctx := xcontext.Detach(ctx)
			return reply(ctx, nil, protocol.RequestCancelledError)
		}
		handled, err := clientDispatch(ctx, client, reply, req)
		if handled || err != nil {
			return err
		}
		return handler(ctx, reply, req)
	}
}

func ServerHandler(server Server, handler jsonrpc2.Handler) jsonrpc2.Handler {
	return func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		if ctx.Err() != nil {
			ctx := xcontext.Detach(ctx)
			return reply(ctx, nil, protocol.RequestCancelledError)
		}
		handled, err := serverDispatch(ctx, server, reply, req)
		if handled || err != nil {
			return err
		}
		return fmt.Errorf("non-standard request")
	}
}

func ServerDispatcher(conn jsonrpc2.Conn, server protocol.Server) Server {
	return &serverDispatcher{
		Conn:   conn,
		Server: server,
	}
}

func ClientDispatcher(conn jsonrpc2.Conn, client protocol.Client) Client {
	return &clientDispatcher{
		Conn:   conn,
		Client: client,
	}
}

func NewClient(ctx context.Context, conn net.Conn, client Client) Server {
	stream := jsonrpc2.NewHeaderStream(conn)
	cc := jsonrpc2.NewConn(stream)
	server := ServerDispatcher(cc, protocol.ServerDispatcher(cc))
	cc.Go(ctx, protocol.Handlers(
		ClientHandler(client,
			protocol.ClientHandler(&lspClientDispatcher{Client: client},
				jsonrpc2.MethodNotFound))))
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

func (s *streamServer) ServeStream(ctx context.Context, conn jsonrpc2.Conn) error {
	// The proxy server doesn't need to make calls to the client.
	//client := ClientDispatcher(cc, protocol.ClientDispatcher(cc))
	conn.Go(ctx,
		protocol.Handlers(
			ServerHandler(s.server,
				protocol.ServerHandler(&lspServerDispatcher{Server: s.server},
					jsonrpc2.MethodNotFound))))
	<-conn.Done()
	return conn.Err()
}

func sendParseError(ctx context.Context, reply jsonrpc2.Replier, err error) error {
	return reply(ctx, nil, fmt.Errorf("%w: %s", jsonrpc2.ErrParse, err))
}

type ExecuteCommandOnDocumentParams struct {
	TextDocument         protocol.TextDocumentIdentifier
	ExecuteCommandParams protocol.ExecuteCommandParams
}
