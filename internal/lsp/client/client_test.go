package client

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fhs/acme-lsp/internal/lsp"
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

func testGoModule(t *testing.T, server string, src string, f func(t *testing.T, c *Conn, uri lsp.DocumentURI)) {
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
	srv, err := StartServer(args, os.Stdout, dir)
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
		testGoModule(t, server, goSourceUnfmt, func(t *testing.T, c *Conn, uri lsp.DocumentURI) {
			edits, err := c.Format(uri)
			if err != nil {
				t.Fatalf("Format failed: %v", err)
			}
			f := text.BytesFile([]byte(goSourceUnfmt))
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
		testGoModule(t, srv.name, goSource, func(t *testing.T, c *Conn, uri lsp.DocumentURI) {
			pos := &lsp.TextDocumentPositionParams{
				TextDocument: lsp.TextDocumentIdentifier{
					URI: uri,
				},
				Position: lsp.Position{
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

func testPython(t *testing.T, src string, f func(t *testing.T, c *Conn, uri lsp.DocumentURI)) {
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
	srv, err := StartServer([]string{"pyls"}, os.Stdout, dir)
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
	testPython(t, pySource, func(t *testing.T, c *Conn, uri lsp.DocumentURI) {
		pos := &lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{
				URI: uri,
			},
			Position: lsp.Position{
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
	testPython(t, pySourceUnfmt, func(t *testing.T, c *Conn, uri lsp.DocumentURI) {
		edits, err := c.Format(uri)
		if err != nil {
			t.Fatalf("Format failed: %v", err)
		}
		f := text.BytesFile([]byte(pySourceUnfmt))
		err = text.Edit(&f, edits)
		if err != nil {
			t.Fatalf("failed to apply edits: %v", err)
		}
		if got := string(f); got != pySource {
			t.Errorf("bad format output:\n%s\nexpected:\n%s", got, pySource)
		}
	})
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
