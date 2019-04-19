package client

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fhs/acme-lsp/internal/lsp"
)

const goSource = `package main // import "example.com/test"

import "fmt"

func main() {
	fmt.Println("Hello, 世界")
}
`

func testGoHover(t *testing.T, want string, command []string) {
	// Create the module
	dir, err := ioutil.TempDir("", "examplemod")
	if err != nil {
		t.Fatalf("TempDir failed: %v", err)
	}
	defer os.RemoveAll(dir)

	gofile := filepath.Join(dir, "main.go")
	if err := ioutil.WriteFile(gofile, []byte(goSource), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	modfile := filepath.Join(dir, "go.mod")
	if err := ioutil.WriteFile(modfile, nil, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Start the server
	srv, err := StartServer(command, os.Stdout, dir)
	if err != nil {
		t.Fatalf("startServer failed: %v", err)
	}
	defer srv.Close()

	err = srv.Conn.DidOpen(gofile, []byte(goSource))
	if err != nil {
		t.Fatalf("DidOpen failed: %v", err)
	}
	defer func() {
		err := srv.Conn.DidClose(gofile)
		if err != nil {
			t.Fatalf("DidClose failed: %v", err)
		}
	}()

	t.Run("Format", func(t *testing.T) {
		edits, err := srv.Conn.Format(ToURI(gofile))
		if err != nil {
			t.Fatalf("Format failed: %v", err)
		}
		t.Logf("Format returned %v edits\n", len(edits))
	})

	t.Run("Hover", func(t *testing.T) {
		pos := &lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{
				URI: ToURI(gofile),
			},
			Position: lsp.Position{
				Line:      5,
				Character: 10,
			},
		}
		var b bytes.Buffer
		if err := srv.Conn.Hover(pos, &b); err != nil {
			t.Fatalf("Hover failed: %v", err)
		}
		got := b.String()
		// Instead of doing an exact match, we ignore extra markups
		// from markdown (if there are any).
		if !strings.Contains(got, want) {
			t.Errorf("hover result is %q; expected %q", got, want)
		}
	})
}

func TestGopls(t *testing.T) {
	want := "func fmt.Println(a ...interface{}) (n int, err error)\n"
	testGoHover(t, want, []string{"gopls"})
}

func TestGoLangServer(t *testing.T) {
	want := "func Println(a ...interface{}) (n int, err error)\nPrintln formats using the default formats for its operands and writes to standard output. Spaces are always added between operands and a newline is appended. It returns the number of bytes written and any write error encountered. \n\n\n"
	testGoHover(t, want, []string{"go-langserver"})
}

const pySource = `#!/usr/bin/env python

import math

def main():
    print(math.sqrt(42))

if __name__ == '__main__':
    main()
`

func testPythonHover(t *testing.T, want string, command []string) {
	dir, err := ioutil.TempDir("", "lspexample")
	if err != nil {
		t.Fatalf("TempDir failed: %v", err)
	}
	defer os.RemoveAll(dir)

	pyfile := filepath.Join(dir, "main.py")
	if err := ioutil.WriteFile(pyfile, []byte(pySource), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Start the server
	srv, err := StartServer(command, os.Stdout, dir)
	if err != nil {
		t.Fatalf("startServer failed: %v", err)
	}
	defer srv.Close()

	err = srv.Conn.DidOpen(pyfile, []byte(pySource))
	if err != nil {
		t.Fatalf("DidOpen failed: %v", err)
	}
	defer func() {
		err := srv.Conn.DidClose(pyfile)
		if err != nil {
			t.Fatalf("DidClose failed: %v", err)
		}
	}()

	t.Run("Format", func(t *testing.T) {
		edits, err := srv.Conn.Format(ToURI(pyfile))
		if err != nil {
			t.Fatalf("Format failed: %v", err)
		}
		t.Logf("Format returned %v edits\n", len(edits))
	})

	t.Run("Hover", func(t *testing.T) {
		pos := &lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{
				URI: ToURI(pyfile),
			},
			Position: lsp.Position{
				Line:      5,
				Character: 16,
			},
		}
		var b bytes.Buffer
		if err := srv.Conn.Hover(pos, &b); err != nil {
			t.Fatalf("Hover failed: %v", err)
		}
		got := b.String()
		// May not be an exact match.
		// Perhaps depending on if it's Python 2 or 3?
		if !strings.Contains(got, want) {
			t.Errorf("hover result is %q does not contain %q", got, want)
		}
	})
}

func TestPyls(t *testing.T) {
	want := "Return the square root of x.\n"
	testPythonHover(t, want, []string{"pyls"})
}

func TestURI(t *testing.T) {
	for _, tc := range []struct {
		name string
		uri  lsp.DocumentURI
	}{
		{"/home/gopher/hello.go", "file:///home/gopher/hello.go"},
	} {
		uri := ToURI(tc.name)
		if uri != tc.uri {
			t.Errorf("ToURI(%q) is %q; expected %q", tc.name, uri, tc.uri)
		}
		name := ToPath(tc.uri)
		if name != tc.name {
			t.Errorf("ToPath(%q) is %q; expected %q", tc.uri, name, tc.name)
		}
	}
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
