// Package lsp contains Language Server Protocol utility functions.
package lsp

import (
	"context"
	"fmt"
	"log"
	"path/filepath"

	"github.com/fhs/acme-lsp/internal/golang_org_x_tools/lsp/protocol"
	"github.com/fhs/acme-lsp/internal/lsp/text"
)

func ServerProvidesCodeAction(cap *protocol.ServerCapabilities, kind protocol.CodeActionKind) bool {
	switch ap := cap.CodeActionProvider.(type) {
	case bool:
		return ap
	case map[string]interface{}:
		opt, err := protocol.ToCodeActionOptions(ap)
		if err != nil {
			log.Printf("failed to decode CodeActionOptions: %v", err)
			return false
		}
		for _, k := range opt.CodeActionKinds {
			if k == kind {
				return true
			}
		}
	}
	return false
}

func CompatibleCodeActions(cap *protocol.ServerCapabilities, kinds []protocol.CodeActionKind) []protocol.CodeActionKind {
	switch ap := cap.CodeActionProvider.(type) {
	case bool:
		if ap {
			return kinds
		}
		return nil
	case map[string]interface{}:
		opt, err := protocol.ToCodeActionOptions(ap)
		if err != nil {
			log.Printf("failed to decode CodeActionOptions: %v", err)
			return nil
		}
		var compat []protocol.CodeActionKind
		for _, k := range kinds {
			found := false
			for _, kk := range opt.CodeActionKinds {
				if k == kk {
					found = true
					break
				}
			}
			if found {
				compat = append(compat, k)
			} else {
				log.Printf("code action %v is not compatible with server", k)
			}
		}
		return compat
	}
	return nil
}

func LocationLink(l *protocol.Location) string {
	p := text.ToPath(l.URI)
	return fmt.Sprintf("%s:%v:%v-%v:%v", p,
		l.Range.Start.Line+1, l.Range.Start.Character+1,
		l.Range.End.Line+1, l.Range.End.Character+1)
}

func DidOpen(ctx context.Context, server protocol.Server, filename string, lang string, body []byte) error {
	if lang == "" {
		lang = DetectLanguage(filename)
	}
	return server.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        text.ToURI(filename),
			LanguageID: lang,
			Version:    0,
			Text:       string(body),
		},
	})
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
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{
				URI: text.ToURI(filename),
			},
			// TODO(fhs): add text field for includeText option
		},
	})
}

func DidChange(ctx context.Context, server protocol.Server, filename string, body []byte) error {
	return server.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{
				URI: text.ToURI(filename),
			},
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			{
				Text: string(body),
			},
		},
	})
}

func DetectLanguage(filename string) string {
	switch base := filepath.Base(filename); base {
	case "go.mod", "go.sum":
		return base
	}
	lang := filepath.Ext(filename)
	if len(lang) == 0 {
		return lang
	}
	if lang[0] == '.' {
		lang = lang[1:]
	}
	switch lang {
	case "py":
		lang = "python"
	}
	return lang
}

func DirsToWorkspaceFolders(dirs []string) ([]protocol.WorkspaceFolder, error) {
	var workspaces []protocol.WorkspaceFolder
	for _, d := range dirs {
		d, err := filepath.Abs(d)
		if err != nil {
			return nil, err
		}
		workspaces = append(workspaces, protocol.WorkspaceFolder{
			URI:  text.ToURI(d),
			Name: d,
		})
	}
	return workspaces, nil
}
