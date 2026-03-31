// Package lsp contains Language Server Protocol utility functions.
package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"sync"

	"9fans.net/acme-lsp/internal/lsp/proxy"
	"9fans.net/acme-lsp/internal/lsp/text"
	"9fans.net/internal/go-lsp/lsp/protocol"
	"github.com/sourcegraph/jsonrpc2"
)

func ServerProvidesCodeAction(cap *protocol.ServerCapabilities, kind protocol.CodeActionKind) bool {
	switch ap := cap.CodeActionProvider.(type) {
	case bool:
		return ap
	case protocol.CodeActionOptions:
		for _, k := range ap.CodeActionKinds {
			if k == kind {
				return true
			}
		}
	}
	return false
}

func CompatibleCodeActions(cap *protocol.ServerCapabilities, kinds []protocol.CodeActionKind) []protocol.CodeActionKind {
	var allowed []protocol.CodeActionKind
	switch ap := cap.CodeActionProvider.(type) {
	default:
		log.Printf("CompatibleCodeActions: unexpected CodeActionProvider type %T", ap)
	case bool:
		if ap {
			return kinds
		}
		return nil
	case protocol.CodeActionOptions:
		allowed = ap.CodeActionKinds
	case map[string]any:
		as, ok := ap["codeActionKinds"].([]any)
		if !ok {
			log.Printf("codeActionKinds is %T", ap["codeActionKinds"])
			break
		}
		for i, a := range as {
			b, ok := a.(string)
			if !ok {
				log.Printf("codeActionKinds[%d] is %T", i, b)
			}
			allowed = append(allowed, protocol.CodeActionKind(b))
		}
	}

	var compat []protocol.CodeActionKind
Kinds:
	for _, k := range kinds {
		for _, allow := range allowed {
			if k == allow {
				compat = append(compat, k)
				continue Kinds
			}
		}
		log.Printf("code action %v is not compatible with server kinds %v", k, allowed)
	}
	return compat

}

func ServerSupportsIncrementalSync(cap *protocol.ServerCapabilities) bool {
	switch v := cap.TextDocumentSync.(type) {
	case float64:
		return protocol.TextDocumentSyncKind(v) == protocol.Incremental
	case protocol.TextDocumentSyncKind:
		return v == protocol.Incremental
	case protocol.TextDocumentSyncOptions:
		return v.Change == protocol.Incremental
	case map[string]any:
		change, ok := v["change"].(float64)
		return ok && protocol.TextDocumentSyncKind(change) == protocol.Incremental
	}
	return false
}

func LocationLink(l *protocol.Location, basedir string) string {
	p := text.ToPath(l.URI)
	rel, err := filepath.Rel(basedir, p)
	if err == nil && len(rel) < len(p) {
		p = rel
	}
	return fmt.Sprintf("%s:%v.%v,%v.%v", p,
		l.Range.Start.Line+1, l.Range.Start.Character+1,
		l.Range.End.Line+1, l.Range.End.Character+1)
}

func DidClose(ctx context.Context, server protocol.Server, filename string) error {
	return server.DidClose(ctx, &protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: text.ToURI(filename),
		},
	})
}

func DidSave(ctx context.Context, server protocol.Server, filename string) error {
	return server.DidSave(ctx, &protocol.DidSaveTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: text.ToURI(filename),
			// TODO(fhs): add text field for includeText option
		},
	})
}

func SyncDocument(ctx context.Context, server proxy.Server, filename string, body []byte) error {
	return server.SyncDocument(ctx, &proxy.SyncDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: text.ToURI(filename),
		},
		Content: string(body),
	})
}

func DetectLanguage(filename string) protocol.LanguageKind {
	switch base := filepath.Base(filename); base {
	case "go.mod", "go.sum":
		return protocol.LanguageKind(base)
	}
	lang := filepath.Ext(filename)
	if len(lang) == 0 {
		return protocol.LanguageKind(lang)
	}
	if lang[0] == '.' {
		lang = lang[1:]
	}
	switch lang {
	case "cs":
		lang = "csharp"
	case "py":
		lang = "python"
	case "ts":
		lang = "typescript"
	}
	return protocol.LanguageKind(lang)
}

func DirsToWorkspaceFolders(dirs []string) ([]protocol.WorkspaceFolder, error) {
	var workspaces []protocol.WorkspaceFolder
	for _, d := range dirs {
		d, err := filepath.Abs(d)
		if err != nil {
			return nil, err
		}
		workspaces = append(workspaces, protocol.WorkspaceFolder{
			URI:  string(text.ToURI(d)),
			Name: d,
		})
	}
	return workspaces, nil
}

// LogMessages causes all messages sent and received on conn to be
// logged using the provided logger.
//
// This works around a bug in jsonrpc2.
// Upstream PR: https://github.com/sourcegraph/jsonrpc2/pull/71
func LogMessages(logger jsonrpc2.Logger) jsonrpc2.ConnOpt {
	return func(c *jsonrpc2.Conn) {
		// Remember reqs we have received so we can helpfully show the
		// request method in OnSend for responses.
		var (
			mu         sync.Mutex
			reqMethods = map[jsonrpc2.ID]string{}
		)

		jsonrpc2.OnRecv(func(req *jsonrpc2.Request, resp *jsonrpc2.Response) {
			switch {
			case resp != nil:
				var method string
				if req != nil {
					method = req.Method
				} else {
					method = "(no matching request)"
				}
				switch {
				case resp.Result != nil:
					result, _ := json.Marshal(resp.Result)
					logger.Printf("jsonrpc2: --> result #%s: %s: %s\n", resp.ID, method, result)
				case resp.Error != nil:
					err, _ := json.Marshal(resp.Error)
					logger.Printf("jsonrpc2: --> error #%s: %s: %s\n", resp.ID, method, err)
				}

			case req != nil:
				mu.Lock()
				reqMethods[req.ID] = req.Method
				mu.Unlock()

				params, _ := json.Marshal(req.Params)
				if req.Notif {
					logger.Printf("jsonrpc2: --> notif: %s: %s\n", req.Method, params)
				} else {
					logger.Printf("jsonrpc2: --> request #%s: %s: %s\n", req.ID, req.Method, params)
				}
			}
		})(c)
		jsonrpc2.OnSend(func(req *jsonrpc2.Request, resp *jsonrpc2.Response) {
			switch {
			case resp != nil:
				mu.Lock()
				method := reqMethods[resp.ID]
				delete(reqMethods, resp.ID)
				mu.Unlock()
				if method == "" {
					method = "(no previous request)"
				}

				if resp.Result != nil {
					result, _ := json.Marshal(resp.Result)
					logger.Printf("jsonrpc2: <-- result #%s: %s: %s\n", resp.ID, method, result)
				} else {
					err, _ := json.Marshal(resp.Error)
					logger.Printf("jsonrpc2: <-- error #%s: %s: %s\n", resp.ID, method, err)
				}

			case req != nil:
				params, _ := json.Marshal(req.Params)
				if req.Notif {
					logger.Printf("jsonrpc2: <-- notif: %s: %s\n", req.Method, params)
				} else {
					logger.Printf("jsonrpc2: <-- request #%s: %s: %s\n", req.ID, req.Method, params)
				}
			}
		})(c)
	}
}
