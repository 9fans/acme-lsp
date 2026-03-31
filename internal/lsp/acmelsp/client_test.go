package acmelsp

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"9fans.net/acme-lsp/internal/lsp"
	"9fans.net/acme-lsp/internal/lsp/acmelsp/config"
	"9fans.net/acme-lsp/internal/lsp/text"
	"9fans.net/internal/go-lsp/lsp/protocol"
)

const goSource = `package main // import "example.com/test"

import "fmt"

func main() {
	fmt.Println("Hello, 世界")
}
`

const goSourceUnfmt = `package main // import "example.com/test"

import "fmt"

func main( ){
fmt . Println	( "Hello, 世界" )
}
`

const goMod = `module 9fans.net/acme-lsp/internal/lsp/acmelsp/client_test
`

func testGoModule(t *testing.T, server string, src string, f func(t *testing.T, c *Client, uri protocol.DocumentURI)) {
	serverArgs := map[string][]string{
		"gopls": {"gopls"},
	}

	// Create the module
	dir, err := ioutil.TempDir("", "examplemod")
	if err != nil {
		t.Fatalf("TempDir failed: %v", err)
	}
	defer os.RemoveAll(dir)

	gofile := filepath.Join(dir, "main.go")
	if err := ioutil.WriteFile(gofile, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	modfile := filepath.Join(dir, "go.mod")
	if err := ioutil.WriteFile(modfile, []byte(goMod), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Start the server
	args, ok := serverArgs[server]
	if !ok {
		t.Fatalf("unknown server %q", server)
	}
	cs := &config.Server{
		Command: args,
	}
	srv, err := execServer(cs, &ClientConfig{
		Server:        &config.Server{},
		RootDirectory: dir,
		DiagWriter:    &mockDiagosticsWriter{ioutil.Discard},
		Workspaces:    nil,
	}, false)
	if err != nil {
		t.Fatalf("startServer failed: %v", err)
	}
	defer srv.Close()

	ctx := context.Background()
	err = lsp.DidOpen(ctx, srv.Client, gofile, "go", []byte(src))
	if err != nil {
		t.Fatalf("DidOpen failed: %v", err)
	}
	defer func() {
		err := lsp.DidClose(ctx, srv.Client, gofile)
		if err != nil {
			t.Fatalf("DidClose failed: %v", err)
		}
		srv.Client.Close()
	}()

	t.Run(server, func(t *testing.T) {
		f(t, srv.Client, text.ToURI(gofile))
	})
}

func TestGoFormat(t *testing.T) {
	ctx := context.Background()

	for _, server := range []string{
		"gopls",
	} {
		testGoModule(t, server, goSourceUnfmt, func(t *testing.T, c *Client, uri protocol.DocumentURI) {
			edits, err := c.Formatting(ctx, &protocol.DocumentFormattingParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: uri,
				},
			})
			if err != nil {
				t.Fatalf("Format failed: %v", err)
			}
			f := BytesFile([]byte(goSourceUnfmt))
			err = text.Edit(&f, edits)
			if err != nil {
				t.Fatalf("failed to apply edits: %v", err)
			}
			if got := string(f); got != goSource {
				t.Errorf("bad format output:\n%s\nexpected:\n%s", got, goSource)
			}
		})
	}
}

func TestGoHover(t *testing.T) {
	ctx := context.Background()

	for _, srv := range []struct {
		name string
		want string
	}{
		{"gopls", "```go\nfunc fmt.Println(a ...any) (n int, err error)\n```\n\n---\n\nPrintln formats using the default formats for its operands and writes to standard output. Spaces are always added between operands and a newline is appended. It returns the number of bytes written and any write error encountered.\n\n\n---\n\n[`fmt.Println` on pkg.go.dev](https://pkg.go.dev/fmt#Println)"},
	} {
		testGoModule(t, srv.name, goSource, func(t *testing.T, c *Client, uri protocol.DocumentURI) {
			pos := &protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: uri,
				},
				Position: protocol.Position{
					Line:      5,
					Character: 10,
				},
			}
			hov, err := c.Hover(ctx, &protocol.HoverParams{
				TextDocumentPositionParams: *pos,
			})
			if err != nil {
				t.Fatalf("Hover failed: %v", err)
			}
			markedString, ok := hov.Contents.Value.(protocol.MarkedString)
			if !ok {
				t.Fatalf("hover result %T is not a MarkedString", hov.Contents.Value)
			}
			got := markedString.Value.(protocol.MarkedStringWithLanguage).Value

			// Instead of doing an exact match, we ignore extra markups
			// from markdown (if there are any).
			if !strings.Contains(got, srv.want) {
				t.Errorf("hover result is %q; expected %q", got, srv.want)
			}
		})
	}
}

func TestGoDefinition(t *testing.T) {
	src := `package main // import "example.com/test"

import "fmt"

func hello() string { return "Hello" }

func main() {
	fmt.Printf("%v\n", hello())
}
`

	for _, srv := range []string{
		"gopls",
	} {
		testGoModule(t, srv, src, func(t *testing.T, c *Client, uri protocol.DocumentURI) {
			params := &protocol.DefinitionParams{
				TextDocumentPositionParams: protocol.TextDocumentPositionParams{
					TextDocument: protocol.TextDocumentIdentifier{
						URI: uri,
					},
					Position: protocol.Position{
						Line:      7,
						Character: 22,
					},
				},
			}
			got, err := c.Definition(context.Background(), params)
			if err != nil {
				t.Fatalf("Definition failed: %v", err)
			}
			want := []protocol.Location{
				{
					URI: uri,
					Range: protocol.Range{
						Start: protocol.Position{
							Line:      4,
							Character: 5,
						},
						End: protocol.Position{
							Line:      4,
							Character: 10,
						},
					},
				},
			}
			if !reflect.DeepEqual(want, locationsFromDefinition(got)) {
				t.Errorf("definition result is %v; expected %v", got, want)
			}
		})
	}
}

func TestGoTypeDefinition(t *testing.T) {
	ctx := context.Background()
	src := `package main // import "example.com/test"

import "fmt"

type T string

func main() {
	foo := T("hello")
	fmt.Printf("%v\n", foo)
}
`

	for _, srv := range []string{
		"gopls",
	} {
		testGoModule(t, srv, src, func(t *testing.T, c *Client, uri protocol.DocumentURI) {
			pos := &protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: uri,
				},
				Position: protocol.Position{
					Line:      7,
					Character: 2,
				},
			}
			result, err := c.TypeDefinition(ctx, &protocol.TypeDefinitionParams{
				TextDocumentPositionParams: *pos,
			})
			if err != nil {
				t.Fatalf("TypeDefinition failed: %v", err)
			}
			got := locationsFromTypeDefinition(result)
			want := []protocol.Location{
				{
					URI: uri,
					Range: protocol.Range{
						Start: protocol.Position{
							Line:      4,
							Character: 5,
						},
						End: protocol.Position{
							Line:      4,
							Character: 6,
						},
					},
				},
			}
			if !reflect.DeepEqual(want, got) {
				t.Errorf("type definition result is %v; expected %v", got, want)
			}
		})
	}
}

func TestGoDiagnostics(t *testing.T) {
	src := `package main // import "example.com/test"

func main() {
	var s string
}
`
	dir, err := ioutil.TempDir("", "examplemod")
	if err != nil {
		t.Fatalf("TempDir failed: %v", err)
	}
	defer os.RemoveAll(dir)

	gofile := filepath.Join(dir, "main.go")
	if err := ioutil.WriteFile(gofile, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	modfile := filepath.Join(dir, "go.mod")
	if err := ioutil.WriteFile(modfile, []byte(goMod), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	ch := make(chan *protocol.Diagnostic)
	cs := &config.Server{
		Command: []string{"gopls"},
	}
	srv, err := execServer(cs, &ClientConfig{
		Server:        &config.Server{},
		RootDirectory: dir,
		DiagWriter:    &chanDiagosticsWriter{ch},
		Workspaces:    nil,
	}, false)
	if err != nil {
		t.Fatalf("startServer failed: %v", err)
	}
	defer srv.Close()

	ctx := context.Background()
	err = lsp.DidOpen(ctx, srv.Client, gofile, "go", []byte(src))
	if err != nil {
		t.Fatalf("DidOpen failed: %v", err)
	}

	diag := <-ch
	wantMessage := "declared and not used: s"
	if diag.Message != wantMessage {
		t.Errorf("diagnostics message is %q; expected %q", diag.Message, wantMessage)
	}

	err = lsp.DidClose(ctx, srv.Client, gofile)
	if err != nil {
		t.Fatalf("DidClose failed: %v", err)
	}
	srv.Client.Close()
}

func TestFileLanguage(t *testing.T) {
	for _, tc := range []struct {
		name string
		lang protocol.LanguageKind
	}{
		{"/home/gopher/hello.py", "python"},
		{"/home/gopher/hello.go", "go"},
		{"/home/gopher/go.mod", "go.mod"},
		{"/home/gopher/go.sum", "go.sum"},
		{"/home/gopher/.config/acme-lsp/config.toml", "toml"},
	} {
		lang := lsp.DetectLanguage(tc.name)
		if lang != tc.lang {
			t.Errorf("language ID of %q is %q; expected %q", tc.name, lang, tc.lang)
		}
	}
}

func TestLocationLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TODO: failing on windows due to file path issues")
	}

	l := &protocol.Location{
		URI: protocol.DocumentURI("file:///home/gopher/mod1/main.go"),
		Range: protocol.Range{
			Start: protocol.Position{
				Line:      13,
				Character: 9,
			},
			End: protocol.Position{
				Line:      15,
				Character: 7,
			},
		},
	}
	got := lsp.LocationLink(l, "")
	want := "/home/gopher/mod1/main.go:14.10,16.8"
	if got != want {
		t.Errorf("LocationLink(%v) returned %q; want %q", l, got, want)
	}
}

type chanDiagosticsWriter struct {
	ch chan *protocol.Diagnostic
}

func (dw *chanDiagosticsWriter) WriteDiagnostics(params *protocol.PublishDiagnosticsParams) {
	for _, diag := range params.Diagnostics {
		dw.ch <- &diag
	}
}

var _ = text.File((*BytesFile)(nil))

type BytesFile []byte

func (f *BytesFile) Reader() (io.Reader, error) {
	return bytes.NewReader(*f), nil
}

func (f *BytesFile) WriteAt(q0, q1 int, b []byte) (int, error) {
	r := []rune(string(*f))

	rr := make([]rune, 0, len(r)+len(b))
	rr = append(rr, r[:q0]...)
	rr = append(rr, []rune(string(b))...)
	rr = append(rr, r[q1:]...)
	*f = []byte(string(rr))
	return len(b), nil
}

func (f *BytesFile) Mark() error {
	return nil
}

func (f *BytesFile) DisableMark() error {
	return nil
}

func (f *BytesFile) CloseFiles() {}

func TestClientProvidesCodeAction(t *testing.T) {
	for _, tc := range []struct {
		provider interface{}
		kind     protocol.CodeActionKind
		want     bool
	}{
		{true, protocol.SourceOrganizeImports, true},
		{false, protocol.SourceOrganizeImports, false},
		{false, protocol.SourceOrganizeImports, false},
		{
			protocol.CodeActionOptions{CodeActionKinds: []protocol.CodeActionKind{protocol.QuickFix, protocol.SourceOrganizeImports}},
			protocol.SourceOrganizeImports,
			true,
		},
		{
			protocol.CodeActionOptions{CodeActionKinds: []protocol.CodeActionKind{protocol.QuickFix}},
			protocol.SourceOrganizeImports,
			false,
		},
		{
			protocol.CodeActionOptions{CodeActionKinds: []protocol.CodeActionKind{}},
			protocol.SourceOrganizeImports,
			false,
		},
		{
			protocol.CodeActionOptions{CodeActionKinds: nil},
			protocol.SourceOrganizeImports,
			false,
		},
	} {
		c := &Client{
			initializeResult: &protocol.InitializeResult{
				Capabilities: protocol.ServerCapabilities{
					CodeActionProvider: tc.provider,
				},
			},
		}
		got := lsp.ServerProvidesCodeAction(&c.initializeResult.Capabilities, tc.kind)
		want := tc.want
		if got != want {
			t.Errorf("got %v for provider %v and kind %v; want %v",
				got, tc.provider, tc.kind, want)
		}
	}
}
