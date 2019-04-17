package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
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
	*rootdir = dir
	srv, err := startServer(command)
	if err != nil {
		t.Fatalf("startServer failed: %v", err)
	}
	defer srv.Close()

	err = srv.lsp.DidOpen(gofile, []byte(goSource))
	if err != nil {
		t.Fatalf("DidOpen failed: %v", err)
	}
	defer func() {
		err := srv.lsp.DidClose(gofile)
		if err != nil {
			t.Fatalf("DidClose failed: %v", err)
		}
	}()

	pos := &lsp.TextDocumentPositionParams{
		TextDocument: lsp.TextDocumentIdentifier{
			URI: lsp.DocumentURI(filenameToURI(gofile)),
		},
		Position: lsp.Position{
			Line:      5,
			Character: 10,
		},
	}
	var b bytes.Buffer
	if err := srv.lsp.Hover(pos, &b); err != nil {
		t.Fatalf("Hover failed: %v", err)
	}
	got := b.String()
	if want != got {
		t.Errorf("hover result is %q; expected %q", got, want)
	}
}

func TestGopls(t *testing.T) {
	want := "```go\nfunc fmt.Println(a ...interface{}) (n int, err error)\n```\n"
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
	*debug = true

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
	srv, err := startServer(command)
	if err != nil {
		t.Fatalf("startServer failed: %v", err)
	}
	defer srv.Close()

	err = srv.lsp.DidOpen(pyfile, []byte(pySource))
	if err != nil {
		t.Fatalf("DidOpen failed: %v", err)
	}
	defer func() {
		err := srv.lsp.DidClose(pyfile)
		if err != nil {
			t.Fatalf("DidClose failed: %v", err)
		}
	}()

	pos := &lsp.TextDocumentPositionParams{
		TextDocument: lsp.TextDocumentIdentifier{
			URI: lsp.DocumentURI(filenameToURI(pyfile)),
		},
		Position: lsp.Position{
			Line:      5,
			Character: 16,
		},
	}
	var b bytes.Buffer
	if err := srv.lsp.Hover(pos, &b); err != nil {
		t.Fatalf("Hover failed: %v", err)
	}
	got := b.String()
	if want != got {
		t.Errorf("hover result is %q; expected %q", got, want)
	}
}

func TestPyls(t *testing.T) {
	want := "Return the square root of x.\n"
	testPythonHover(t, want, []string{"pyls"})
}
