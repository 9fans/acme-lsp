package acmelsp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	p9client "9fans.net/go/plan9/client"
	"github.com/fhs/acme-lsp/internal/jsonrpc2"
	"github.com/fhs/acme-lsp/internal/lsp/client"
	"github.com/pkg/errors"
)

type ProxyMessage struct {
	Data string
	Attr map[string]string
}

type proxyHandler struct {
	jsonrpc2.EmptyHandler

	ss  *client.ServerSet // client connections to upstream LSP server (e.g. gopls)
	fm  *FileManager
	rpc *jsonrpc2.Conn // listen for requests on this connection
	log *log.Logger
}

func (h *proxyHandler) Deliver(ctx context.Context, req *jsonrpc2.Request, delivered bool) bool {
	switch req.Method {
	case "acme-lsp/rpc":
		var msg ProxyMessage
		err := json.Unmarshal(*req.Params, &msg)
		if err != nil {
			h.log.Printf("could not unmarshal request params in proxy: %v", err)
			return true
		}

		err = runRPC(h.ss, h.fm, msg.Data, msg.Attr)
		err = req.Reply(ctx, nil, err)
		if err != nil {
			h.log.Printf("could not reply to request: %v", err)
		}
		return true
	}
	return false
}

func ListenAndServeProxy(ctx context.Context, ss *client.ServerSet, fm *FileManager) error {
	ln, err := Listen("unix", ProxyAddr())
	if err != nil {
		return err
	}
	logger := log.New(os.Stderr, "", log.LstdFlags)
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		stream := jsonrpc2.NewHeaderStream(conn, conn)
		rpc := jsonrpc2.NewConn(stream)
		rpc.AddHandler(&proxyHandler{
			ss:  ss,
			fm:  fm,
			rpc: rpc,
			log: logger,
		})
		go rpc.Run(ctx)
	}
}

func ProxyAddr() string {
	return filepath.Join(p9client.Namespace(), "acme-lsp.rpc")
}

func runRPC(ss *client.ServerSet, fm *FileManager, data string, attr map[string]string) error {
	args := strings.Fields(data)
	switch args[0] {
	case "workspaces":
		fmt.Printf("Workspaces:\n")
		for _, d := range ss.Workspaces() {
			fmt.Printf(" %v\n", d)
		}
		return nil
	case "workspaces-add":
		return ss.AddWorkspaces(args[1:])
	case "workspaces-remove":
		return ss.RemoveWorkspaces(args[1:])
	}

	winid, err := strconv.Atoi(attr["winid"])
	if err != nil {
		return errors.Wrap(err, "failed to parse $winid")
	}
	cmd, err := WindowCmd(ss, fm, winid)
	if err != nil {
		return err
	}
	defer cmd.Close()

	switch args[0] {
	case "completion":
		return cmd.Completion(false)
	case "completion-edit":
		return cmd.Completion(true)
	case "definition":
		return cmd.Definition()
	case "type-definition":
		return cmd.TypeDefinition()
	case "format":
		return cmd.Format()
	case "hover":
		return cmd.Hover()
	case "implementation":
		return cmd.Implementation()
	case "references":
		return cmd.References()
	case "rename":
		return cmd.Rename(attr["newname"])
	case "signature":
		return cmd.SignatureHelp()
	case "symbols":
		return cmd.Symbols()
	case "watch-completion":
		go Assist(ss, fm, "comp")
		return nil
	case "watch-signature":
		go Assist(ss, fm, "sig")
		return nil
	case "watch-hover":
		go Assist(ss, fm, "hov")
		return nil
	case "watch-auto":
		go Assist(ss, fm, "auto")
		return nil
	}
	return fmt.Errorf("unknown command %v", args[0])
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
