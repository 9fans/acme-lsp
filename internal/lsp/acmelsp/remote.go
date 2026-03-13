package acmelsp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"9fans.net/acme-lsp/internal/acme"
	"9fans.net/acme-lsp/internal/acmeutil"
	"9fans.net/acme-lsp/internal/lsp"
	"9fans.net/acme-lsp/internal/lsp/proxy"
	"9fans.net/acme-lsp/internal/lsp/text"
	"9fans.net/internal/go-lsp/lsp/protocol"
	p9client "github.com/fhs/9fans-go/plan9/client"
)

// RemoteCmd executes LSP commands in an acme window using the proxy server.
type RemoteCmd struct {
	server proxy.Server
	win    text.AddressableFile
	menu   text.Menu
	Stdout io.Writer
	Stderr io.Writer
}

func NewRemoteCmd(server proxy.Server, win text.AddressableFile, menu text.Menu) *RemoteCmd {
	return &RemoteCmd{
		server: server,
		win:    win,
		menu:   menu,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

func (rc *RemoteCmd) DidOpen(ctx context.Context) error {
	uri, _, err := text.DocumentURI(rc.win)
	if err != nil {
		return err
	}
	filename, err := rc.win.Filename()
	if err != nil {
		return err
	}
	r, err := rc.win.Reader()
	if err != nil {
		return err
	}
	body, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	return rc.server.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        uri,
			LanguageID: lsp.DetectLanguage(filename),
			Version:    0,
			Text:       string(body),
		},
	})
}

func (rc *RemoteCmd) DidChange(ctx context.Context) error {
	uri, _, err := text.DocumentURI(rc.win)
	if err != nil {
		return err
	}
	r, err := rc.win.Reader()
	if err != nil {
		return err
	}
	body, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	return rc.server.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{
				URI: uri,
			},
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			{
				Text: string(body),
			},
		},
	})
}

type CompletionKind int

const (
	CompleteNoEdit CompletionKind = iota
	CompleteInsertOnlyMatch
	CompleteInsertFirstMatch
)

func (rc *RemoteCmd) Completion(ctx context.Context, kind CompletionKind) error {
	pos, _, err := text.Position(rc.win)
	if err != nil {
		return err
	}
	result, err := rc.server.Completion(ctx, &protocol.CompletionParams{
		TextDocumentPositionParams: *pos,
	})
	if err != nil {
		return err
	}
	if result == nil || len(result.Items) == 0 {
		return rc.showCompletion("no completion\n", kind)
	}

	if (kind == CompleteInsertFirstMatch && len(result.Items) >= 1) || (kind == CompleteInsertOnlyMatch && len(result.Items) == 1) {
		textEdit := result.Items[0].TextEdit
		switch edit := textEdit.Value.(type) {
		default:
			return fmt.Errorf("unsupported completion text edit %T", edit)
		case nil:
			// TODO(fhs): Use insertText or label instead.
			return fmt.Errorf("nil TextEdit in completion item")
		case protocol.TextEdit:
			if err := text.Edit(rc.win, []protocol.TextEdit{edit}); err != nil {
				return fmt.Errorf("failed to apply completion edit: %v", err)
			}
			if len(result.Items) == 1 {
				return nil
			}
		}
	}

	var sb strings.Builder
	for _, item := range result.Items {
		fmt.Fprintf(&sb, "%v\t%v\n", item.Label, item.Detail)
	}
	return rc.showCompletion(sb.String(), kind)
}

func (rc *RemoteCmd) showCompletion(body string, kind CompletionKind) error {
	if kind == CompleteInsertFirstMatch {
		cw, err := acmeutil.Hijack("/LSP/Completions")
		if err != nil {
			cw, err = acmeutil.NewWin()
			if err != nil {
				return err
			}
			cw.Name("/LSP/Completions")
		}
		defer cw.Win.Ctl("clean")

		cw.Clear()
		cw.PrintTabbed(body)
		return nil
	}
	fmt.Fprintln(rc.Stdout, body)
	return nil
}

func (rc *RemoteCmd) Definition(ctx context.Context, print bool) error {
	pos, _, err := text.Position(rc.win)
	if err != nil {
		return fmt.Errorf("failed to get position: %v", err)
	}
	locations, err := rc.server.Definition(ctx, &protocol.DefinitionParams{
		TextDocumentPositionParams: *pos,
	})
	if err != nil {
		return fmt.Errorf("bad server response: %v", err)
	}
	if len(locations) == 0 {
		return fmt.Errorf("no definition found")
	}
	if print {
		return PrintLocations(rc.Stdout, locations)
	}
	return PlumbLocations(locations)
}

func (rc *RemoteCmd) OrganizeImportsAndFormat(ctx context.Context) error {
	uri, _, err := text.DocumentURI(rc.win)
	if err != nil {
		return err
	}

	doc := &protocol.TextDocumentIdentifier{
		URI: uri,
	}
	return CodeActionAndFormat(ctx, rc.server, doc, rc.win, rc.menu, []protocol.CodeActionKind{
		protocol.SourceOrganizeImports,
	})
}

func (rc *RemoteCmd) Hover(ctx context.Context) error {
	pos, _, err := text.Position(rc.win)
	if err != nil {
		return err
	}

	hov, err := rc.server.Hover(ctx, &protocol.HoverParams{
		TextDocumentPositionParams: *pos,
	})
	if err != nil {
		return err
	}

	if hov == nil {
		fmt.Fprintln(rc.Stdout, "No hover help available.")
		return nil
	}

	fmt.Fprintf(rc.Stdout, "%v\n", hov.Contents.Value)

	return nil
}

func (rc *RemoteCmd) Implementation(ctx context.Context, print bool) error {
	pos, _, err := text.Position(rc.win)
	if err != nil {
		return err
	}
	loc, err := rc.server.Implementation(ctx, &protocol.ImplementationParams{
		TextDocumentPositionParams: *pos,
	})
	if err != nil {
		return err
	}
	if len(loc) == 0 {
		fmt.Fprintf(rc.Stderr, "No implementations found.\n")
		return nil
	}
	return PrintLocations(rc.Stdout, loc)
}

func (rc *RemoteCmd) References(ctx context.Context) error {
	pos, _, err := text.Position(rc.win)
	if err != nil {
		return err
	}
	loc, err := rc.server.References(ctx, &protocol.ReferenceParams{
		TextDocumentPositionParams: *pos,
		Context: protocol.ReferenceContext{
			IncludeDeclaration: true,
		},
	})
	if err != nil {
		return err
	}
	if len(loc) == 0 {
		fmt.Fprintf(rc.Stderr, "No references found.\n")
		return nil
	}
	return PrintLocations(rc.Stdout, loc)
}

// Rename renames the identifier at cursor position to newname.
func (rc *RemoteCmd) Rename(ctx context.Context, newname string) error {
	pos, _, err := text.Position(rc.win)
	if err != nil {
		return err
	}
	we, err := rc.server.Rename(ctx, &protocol.RenameParams{
		NewName: newname,
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: pos.TextDocument,
			Position:     pos.Position,
		},
	})
	if err != nil {
		return err
	}
	return editWorkspace(we, rc.menu)
}

func (rc *RemoteCmd) SignatureHelp(ctx context.Context) error {
	pos, _, err := text.Position(rc.win)
	if err != nil {
		return err
	}
	sh, err := rc.server.SignatureHelp(ctx, &protocol.SignatureHelpParams{
		TextDocumentPositionParams: *pos,
	})
	if err != nil {
		return err
	}
	if sh != nil {
		for _, sig := range sh.Signatures {
			fmt.Fprintf(rc.Stdout, "%v\n", sig.Label)
			fmt.Fprintf(rc.Stdout, "%v\n", sig.Documentation)
		}
	}
	return nil
}

func (rc *RemoteCmd) DocumentSymbol(ctx context.Context) error {
	basedir := "" // TODO

	uri, _, err := text.DocumentURI(rc.win)
	if err != nil {
		return err
	}

	// TODO(fhs): DocumentSymbol request can return either a
	// []DocumentSymbol (hierarchical) or []SymbolInformation (flat).
	// We only handle the hierarchical type below.

	// TODO(fhs): Make use of DocumentSymbol.Range to optionally filter out
	// symbols that aren't within current cursor position?

	syms, err := rc.server.DocumentSymbol(ctx, &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: uri,
		},
	})
	if err != nil {
		return err
	}
	if len(syms) == 0 {
		fmt.Fprintf(rc.Stderr, "No symbols found.\n")
		return nil
	}
	walkDocumentSymbols(syms, 0, func(s *protocol.DocumentSymbol, depth int) {
		loc := &protocol.Location{
			URI:   uri,
			Range: s.SelectionRange,
		}
		indent := strings.Repeat(" ", depth)
		fmt.Fprintf(rc.Stdout, "%v%v %v\n", indent, s.Name, s.Detail)
		fmt.Fprintf(rc.Stdout, "%v %v\n", indent, lsp.LocationLink(loc, basedir))
	})
	return nil
}

func (rc *RemoteCmd) TypeDefinition(ctx context.Context, print bool) error {
	pos, _, err := text.Position(rc.win)
	if err != nil {
		return err
	}
	locations, err := rc.server.TypeDefinition(ctx, &protocol.TypeDefinitionParams{
		TextDocumentPositionParams: *pos,
	})
	if err != nil {
		return err
	}
	if print {
		return PrintLocations(rc.Stdout, locations)
	}
	return PlumbLocations(locations)
}

func walkDocumentSymbols1(syms []protocol.DocumentSymbol, depth int, f func(s *protocol.DocumentSymbol, depth int)) {
	for _, s := range syms {
		f(&s, depth)
		walkDocumentSymbols1(s.Children, depth+1, f)
	}
}

func walkDocumentSymbols(syms []interface{}, depth int, f func(s *protocol.DocumentSymbol, depth int)) {
	for _, s := range syms {
		switch val := s.(type) {
		default:
			log.Printf("unknown symbol type %T", val)

		case protocol.DocumentSymbol:
			f(&val, depth)
			walkDocumentSymbols1(val.Children, depth+1, f)

		// Workaround for the DocumentSymbol not being parsed by the auto-generated LSP definitions.
		case map[string]interface{}:
			ds, err := parseDocumentSymbol(val)
			if err != nil {
				log.Printf("failed to parse DocumentSymbols: %v\n", err)
			} else {
				f(ds, depth)
				walkDocumentSymbols1(ds.Children, depth+1, f)
			}
		}
	}
}

func parseDocumentSymbol(data map[string]interface{}) (*protocol.DocumentSymbol, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var ds protocol.DocumentSymbol
	if err := json.Unmarshal(b, &ds); err != nil {
		return nil, err
	}
	return &ds, nil
}

func parseAcmeAddr(addr string) (filename string, q0 int, q1 int, err error) {
	f := strings.Split(addr, ":")
	if len(f) < 2 {
		return "", -1, -1, fmt.Errorf("invalid $acmeaddr %q", addr)
	}
	filename = f[0]
	f = strings.Split(f[1], ",")
	if len(f) < 1 {
		return "", -1, -1, fmt.Errorf("invalid $acmeaddr %q", addr)
	}
	q0, err = strconv.Atoi(strings.TrimPrefix(f[0], "#"))
	if err != nil {
		return "", -1, -1, fmt.Errorf("failed to parse q0 in $acmdaddr %q: %v", addr, err)
	}
	q1 = q0
	if len(f) > 1 {
		q1, err = strconv.Atoi(strings.TrimPrefix(f[0], "#"))
		if err != nil {
			return "", -1, -1, fmt.Errorf("failed to parse q1 in $acmdaddr %q: %v", addr, err)
		}
	}
	return filename, q0, q1, nil
}

func getFocusedWinID(addr string) (int, error) {
	winid := os.Getenv("winid")
	if winid == "" {
		conn, err := net.Dial("unix", addr)
		if err != nil {
			return -1, fmt.Errorf("$winid is empty and could not dial acmefocused: %v", err)
		}
		defer conn.Close()
		b, err := io.ReadAll(conn)
		if err != nil {
			return -1, fmt.Errorf("$winid is empty and could not read acmefocused: %v", err)
		}
		winid = string(bytes.TrimSpace(b))
	}
	n, err := strconv.Atoi(winid)
	if err != nil {
		return -1, fmt.Errorf("failed to parse $winid: %v", err)
	}
	return n, nil
}

func OpenFocusedWin(headless bool) (win text.AddressableFile, err error) {
	acmeaddr := os.Getenv("acmeaddr")

	// Headless mode is used for testing.
	// We assume acme is not running and use $acmeaddr to access the file directly on the filesystem.
	if headless {
		filename, q0, q1, err := parseAcmeAddr(acmeaddr)
		if err != nil {
			return nil, fmt.Errorf("failed to to parse $acmeaddr %q: %v", acmeaddr, err)
		}
		return text.NewHeadlessFile(
			filename,
			q0,
			q1,
		)
	}

	// For a 2-1 chord command, $winid may point to the window with the command (e.g. guide file)
	// instead of the target window. Find the correct winid based on $acmeaddr.
	if acmeaddr != "" {
		filename, _, _, err := parseAcmeAddr(acmeaddr)
		if err != nil {
			return nil, fmt.Errorf("failed to to parse $acmeaddr %q: %v", acmeaddr, err)
		}
		// Find the filename in the index
		windows, err := acme.Windows()
		if err != nil {
			return nil, err
		}
		for _, w := range windows {
			if w.Name == filename {
				return acmeutil.OpenWin(w.ID)
			}
		}
		return nil, fmt.Errorf("failed to find window for $acmeaddr %q", acmeaddr)
	}

	// Find windid based on either $winid env variable or acmefocused.
	winid, err := getFocusedWinID(filepath.Join(p9client.Namespace(), "acmefocused"))
	if err != nil {
		return nil, fmt.Errorf("could not get focused window ID: %v", err)
	}
	return acmeutil.OpenWin(winid)
}
