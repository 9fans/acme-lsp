package client

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/fhs/acme-lsp/internal/lsp/protocol"
	"github.com/fhs/acme-lsp/internal/lsp/text"
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

func testGoModule(t *testing.T, server string, src string, f func(t *testing.T, c *Conn, uri protocol.DocumentURI)) {
	serverArgs := map[string][]string{
		"gopls":         {"gopls"},
		"go-langserver": {"go-langserver"},
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
	if err := ioutil.WriteFile(modfile, nil, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Start the server
	args, ok := serverArgs[server]
	if !ok {
		t.Fatalf("unknown server %q", server)
	}
	srv, err := StartServer(args, &Config{
		w:          ioutil.Discard,
		diagWriter: &mockDiagosticsWriter{ioutil.Discard},
		rootdir:    dir,
		workspaces: nil,
	})
	if err != nil {
		t.Fatalf("startServer failed: %v", err)
	}
	defer srv.Close()

	err = srv.Conn.DidOpen(gofile, []byte(src))
	if err != nil {
		t.Fatalf("DidOpen failed: %v", err)
	}
	defer func() {
		err := srv.Conn.DidClose(gofile)
		if err != nil {
			t.Fatalf("DidClose failed: %v", err)
		}
	}()

	t.Run(server, func(t *testing.T) {
		f(t, srv.Conn, text.ToURI(gofile))
	})
}

func TestGoFormat(t *testing.T) {
	for _, server := range []string{
		"gopls",
		"go-langserver",
	} {
		testGoModule(t, server, goSourceUnfmt, func(t *testing.T, c *Conn, uri protocol.DocumentURI) {
			edits, err := c.Format(uri)
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
	for _, srv := range []struct {
		name string
		want string
	}{
		{"gopls", "func fmt.Println(a ...interface{}) (n int, err error)\n"},
		{"go-langserver", "func Println(a ...interface{}) (n int, err error)\nPrintln formats using the default formats for its operands and writes to standard output. Spaces are always added between operands and a newline is appended. It returns the number of bytes written and any write error encountered. \n\n\n"},
	} {
		testGoModule(t, srv.name, goSource, func(t *testing.T, c *Conn, uri protocol.DocumentURI) {
			pos := &protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: uri,
				},
				Position: protocol.Position{
					Line:      5,
					Character: 10,
				},
			}
			var b bytes.Buffer
			if err := c.Hover(pos, &b); err != nil {
				t.Fatalf("Hover failed: %v", err)
			}
			got := b.String()
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
		"go-langserver",
	} {
		testGoModule(t, srv, src, func(t *testing.T, c *Conn, uri protocol.DocumentURI) {
			pos := &protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: uri,
				},
				Position: protocol.Position{
					Line:      7,
					Character: 22,
				},
			}
			got, err := c.Definition(pos)
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
			if !reflect.DeepEqual(want, got) {
				t.Errorf("defintion result is %q; expected %q", got, want)
			}
		})
	}
}

func TestGoTypeDefinition(t *testing.T) {
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
		//"go-langserver", 	// failing
	} {
		testGoModule(t, srv, src, func(t *testing.T, c *Conn, uri protocol.DocumentURI) {
			pos := &protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: uri,
				},
				Position: protocol.Position{
					Line:      7,
					Character: 2,
				},
			}
			got, err := c.TypeDefinition(pos)
			if err != nil {
				t.Fatalf("TypeDefinition failed: %v", err)
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
							Character: 6,
						},
					},
				},
			}
			if !reflect.DeepEqual(want, got) {
				t.Errorf("type defintion result is %q; expected %q", got, want)
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
	if err := ioutil.WriteFile(modfile, nil, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	ch := make(chan *protocol.Diagnostic)
	srv, err := StartServer([]string{"gopls"}, &Config{
		w:          ioutil.Discard,
		diagWriter: &chanDiagosticsWriter{ch},
		rootdir:    dir,
		workspaces: nil,
	})
	if err != nil {
		t.Fatalf("startServer failed: %v", err)
	}
	defer srv.Close()

	err = srv.Conn.DidOpen(gofile, []byte(src))
	if err != nil {
		t.Fatalf("DidOpen failed: %v", err)
	}

	diag := <-ch
	want := "s declared but not used"
	if diag.Message != want {
		t.Errorf("diagnostics message is %q, want %q", diag.Message, want)
	}

	err = srv.Conn.DidClose(gofile)
	if err != nil {
		t.Fatalf("DidClose failed: %v", err)
	}
}

const pySource = `#!/usr/bin/env python

import math


def main():
    print(math.sqrt(42))


if __name__ == '__main__':
    main()
`

const pySourceUnfmt = `#!/usr/bin/env python

import math

def main( ):
    print( math.sqrt ( 42 ) )

if __name__=='__main__':
    main( )
`

func testPython(t *testing.T, src string, f func(t *testing.T, c *Conn, uri protocol.DocumentURI)) {
	dir, err := ioutil.TempDir("", "lspexample")
	if err != nil {
		t.Fatalf("TempDir failed: %v", err)
	}
	defer os.RemoveAll(dir)

	pyfile := filepath.Join(dir, "main.py")
	if err := ioutil.WriteFile(pyfile, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Start the server
	srv, err := StartServer([]string{"pyls"}, &Config{
		w:          ioutil.Discard,
		diagWriter: &mockDiagosticsWriter{ioutil.Discard},
		rootdir:    dir,
		workspaces: nil,
	})
	if err != nil {
		t.Fatalf("startServer failed: %v", err)
	}
	defer srv.Close()

	err = srv.Conn.DidOpen(pyfile, []byte(src))
	if err != nil {
		t.Fatalf("DidOpen failed: %v", err)
	}
	defer func() {
		err := srv.Conn.DidClose(pyfile)
		if err != nil {
			t.Fatalf("DidClose failed: %v", err)
		}
	}()

	f(t, srv.Conn, text.ToURI(pyfile))
}

func TestPythonHover(t *testing.T) {
	testPython(t, pySource, func(t *testing.T, c *Conn, uri protocol.DocumentURI) {
		pos := &protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: uri,
			},
			Position: protocol.Position{
				Line:      6,
				Character: 16,
			},
		}
		var b bytes.Buffer
		if err := c.Hover(pos, &b); err != nil {
			t.Fatalf("Hover failed: %v", err)
		}
		got := b.String()
		want := "Return the square root of x.\n"
		// May not be an exact match.
		// Perhaps depending on if it's Python 2 or 3?
		if !strings.Contains(got, want) {
			t.Errorf("hover result is %q does not contain %q", got, want)
		}
	})
}

func TestPythonFormat(t *testing.T) {
	testPython(t, pySourceUnfmt, func(t *testing.T, c *Conn, uri protocol.DocumentURI) {
		edits, err := c.Format(uri)
		if err != nil {
			t.Fatalf("Format failed: %v", err)
		}
		f := BytesFile([]byte(pySourceUnfmt))
		err = text.Edit(&f, edits)
		if err != nil {
			t.Fatalf("failed to apply edits: %v", err)
		}
		if got := string(f); got != pySource {
			t.Errorf("bad format output:\n%s\nexpected:\n%s", got, pySource)
		}
	})
}

func TestPythonDefinition(t *testing.T) {
	src := `def main():
    pass

if __name__ == '__main__':
    main()
`

	testPython(t, src, func(t *testing.T, c *Conn, uri protocol.DocumentURI) {
		pos := &protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: uri,
			},
			Position: protocol.Position{
				Line:      4,
				Character: 6,
			},
		}
		got, err := c.Definition(pos)
		if err != nil {
			t.Fatalf("Definition failed: %v", err)
		}
		want := []protocol.Location{
			{
				URI: uri,
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      0,
						Character: 4,
					},
					End: protocol.Position{
						Line:      0,
						Character: 8,
					},
				},
			},
		}
		if !reflect.DeepEqual(want, got) {
			t.Errorf("defintion result is %q; expected %q", got, want)
		}
	})
}

func TestPythonTypeDefinition(t *testing.T) {
	t.Logf("pyls doesn't implement LSP textDocument/typeDefinition")
}

func TestFileLanguage(t *testing.T) {
	for _, tc := range []struct {
		name, lang string
	}{
		{"/home/gopher/hello.py", "python"},
		{"/home/gopher/hello.go", "go"},
	} {
		lang := fileLanguage(tc.name)
		if lang != tc.lang {
			t.Errorf("language ID of %q is %q; expected %q", tc.name, lang, tc.lang)
		}
	}
}

func TestLocationLink(t *testing.T) {
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
	got := LocationLink(l)
	want := "/home/gopher/mod1/main.go:14:10-16:8"
	if got != want {
		t.Errorf("LocationLink(%v) returned %q; want %q", l, got, want)
	}
}

type chanDiagosticsWriter struct {
	ch chan *protocol.Diagnostic
}

func (dw *chanDiagosticsWriter) WriteDiagnostics(diags map[protocol.DocumentURI][]protocol.Diagnostic) error {
	for _, uriDiag := range diags {
		for _, diag := range uriDiag {
			dw.ch <- &diag
		}
	}
	return nil
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
