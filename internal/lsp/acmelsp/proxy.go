package acmelsp

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	p9client "9fans.net/go/plan9/client"
	"github.com/fhs/acme-lsp/internal/golang_org_x_tools/jsonrpc2"
	"github.com/fhs/acme-lsp/internal/lsp"
	"github.com/fhs/acme-lsp/internal/lsp/protocol"
	"github.com/fhs/acme-lsp/internal/lsp/proxy"
	"github.com/fhs/acme-lsp/internal/lsp/text"
	"github.com/pkg/errors"
)

type proxyServer struct {
	ss *lsp.ServerSet // client connections to upstream LSP server (e.g. gopls)
	fm *FileManager
}

func (s *proxyServer) SendMessage(ctx context.Context, msg *proxy.Message) error {
	args := strings.Fields(msg.Data)

	winid, err := strconv.Atoi(msg.Attr["winid"])
	if err != nil {
		return errors.Wrap(err, "failed to parse $winid")
	}
	cmd, err := WindowCmd(s.ss, s.fm, winid)
	if err != nil {
		return err
	}
	defer cmd.Close()

	switch args[0] {
	case "type-definition":
		return cmd.TypeDefinition()
	case "format":
		return cmd.Format()
	case "hover":
		return cmd.Hover()
	case "implementation":
		return cmd.Implementation()
	case "rename":
		return cmd.Rename(msg.Attr["newname"])
	case "signature":
		return cmd.SignatureHelp()
	case "symbols":
		return cmd.Symbols()
	case "watch-completion":
		go Assist(s.ss, s.fm, "comp")
		return nil
	case "watch-signature":
		go Assist(s.ss, s.fm, "sig")
		return nil
	case "watch-hover":
		go Assist(s.ss, s.fm, "hov")
		return nil
	case "watch-auto":
		go Assist(s.ss, s.fm, "auto")
		return nil
	}
	return fmt.Errorf("unknown command %v", args[0])
}

func (s *proxyServer) WorkspaceFolders(context.Context) ([]protocol.WorkspaceFolder, error) {
	return s.ss.Workspaces(), nil
}

func (s *proxyServer) DidChangeWorkspaceFolders(ctx context.Context, params *protocol.DidChangeWorkspaceFoldersParams) error {
	return s.ss.DidChangeWorkspaceFolders(params.Event.Added, params.Event.Removed)
}

func (s *proxyServer) Completion(ctx context.Context, params *protocol.CompletionParams) (*protocol.CompletionList, error) {
	srv, err := serverForURI(s.ss, params.TextDocumentPositionParams.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	return srv.Client.Completion(ctx, params)
}

func (s *proxyServer) Definition(ctx context.Context, params *protocol.DefinitionParams) ([]protocol.Location, error) {
	srv, err := serverForURI(s.ss, params.TextDocumentPositionParams.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	return srv.Client.Definition(ctx, params)
}

func (s *proxyServer) References(ctx context.Context, params *protocol.ReferenceParams) ([]protocol.Location, error) {
	srv, err := serverForURI(s.ss, params.TextDocumentPositionParams.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	return srv.Client.References(ctx, params)
}

func serverForURI(ss *lsp.ServerSet, uri protocol.DocumentURI) (*lsp.Server, error) {
	filename := text.ToPath(uri)
	srv, found, err := ss.StartForFile(filename)
	if !found {
		return nil, fmt.Errorf("unknown language server for file %v", filename)
	}
	if err != nil {
		return nil, errors.Wrap(err, "cound not start language server")
	}
	return srv, nil
}

func ListenAndServeProxy(ctx context.Context, ss *lsp.ServerSet, fm *FileManager) error {
	ln, err := Listen("unix", ProxyAddr())
	if err != nil {
		return err
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		stream := jsonrpc2.NewHeaderStream(conn, conn)
		ctx, rpc, _ := proxy.NewServer(ctx, stream, &proxyServer{
			ss: ss,
			fm: fm,
		})
		go rpc.Run(ctx)
	}
}

func ProxyAddr() string {
	return filepath.Join(p9client.Namespace(), "acme-lsp.rpc")
}

// Listen is like net.Listen but it removes dead unix sockets.
func Listen(network, address string) (net.Listener, error) {
	ln, err := net.Listen(network, address)
	if err != nil && network == "unix" && isAddrInUse(err) {
		if _, err1 := net.Dial(network, address); !isConnRefused(err1) {
			return nil, err // Listen error
		}
		// Dead socket, so remove it.
		err = os.Remove(address)
		if err != nil {
			return nil, err
		}
		return net.Listen(network, address)
	}
	return ln, err
}

func isAddrInUse(err error) bool {
	if err, ok := err.(*net.OpError); ok {
		if err, ok := err.Err.(*os.SyscallError); ok {
			return err.Err == syscall.EADDRINUSE
		}
	}
	return false
}

func isConnRefused(err error) bool {
	if err, ok := err.(*net.OpError); ok {
		if err, ok := err.Err.(*os.SyscallError); ok {
			return err.Err == syscall.ECONNREFUSED
		}
	}
	return false
}
