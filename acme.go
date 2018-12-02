package main

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"strconv"

	"9fans.net/go/acme"
	lsp "github.com/sourcegraph/go-lsp"
)

func getAcmePos() (*lsp.TextDocumentPositionParams, string, error) {
	id, err := strconv.Atoi(os.Getenv("winid"))
	if err != nil {
		return nil, "", err
	}
	return getAcmeWinPos(id)
}

func getAcmeWinPos(id int) (*lsp.TextDocumentPositionParams, string, error) {
	w, err := openWin(id)
	if err != nil {
		return nil, "", err
	}
	defer w.CloseFiles()

	q0, _, err := w.ReadDotAddr()
	if err != nil {
		return nil, "", err
	}
	//fmt.Printf("q0=%v q1=%v\n", q0, q1)

	line, col, err := offsetToLine(w.FileReadWriter("body"), q0)
	if err != nil {
		return nil, "", err
	}
	fname, err := w.Filename()
	if err != nil {
		return nil, "", err
	}
	return &lsp.TextDocumentPositionParams{
		TextDocument: lsp.TextDocumentIdentifier{
			URI: lsp.DocumentURI("file://" + fname),
		},
		Position: lsp.Position{
			Line:      line,
			Character: col,
		},
	}, fname, nil
}

// OffsetToLine returns the line number and rune offset within the line
// given rune offset within reader r.
func offsetToLine(rd io.Reader, q int) (line int, col int, err error) {
	br := bufio.NewReader(rd)
	wasnl := true
	line = -1
	for ; q >= 0; q-- {
		r, _, err := br.ReadRune()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, 0, err
		}
		if wasnl {
			line++
			col = 0
			wasnl = false
		} else {
			col++
		}
		if r == '\n' {
			wasnl = true
		}
	}
	if line == -1 { // empty file
		line = 0
	}
	return
}

type win struct {
	*acme.Win
}

func newWin() (*win, error) {
	w, err := acme.New()
	if err != nil {
		return nil, err
	}
	return &win{w}, err
}

func openWin(id int) (*win, error) {
	w, err := acme.Open(id, nil)
	if err != nil {
		return nil, err
	}
	return &win{w}, err
}

func (w *win) Filename() (string, error) {
	tag, err := w.ReadAll("tag")
	if err != nil {
		return "", err
	}
	i := bytes.IndexRune(tag, ' ')
	if i < 0 {
		i = len(tag)
	}
	return string(tag[:i]), nil
}

// ReadDotAddr returns the address of current selection.
func (w *win) ReadDotAddr() (q0, q1 int, err error) {
	_, _, err = w.ReadAddr() // open addr file
	if err != nil {
		return 0, 0, err
	}
	err = w.Ctl("addr=dot")
	if err != nil {
		return 0, 0, err
	}
	return w.ReadAddr()
}

func (w *win) FileReadWriter(filename string) io.ReadWriter {
	return &winFile{
		w:    w.Win,
		name: filename,
	}
}

type winFile struct {
	w    *acme.Win
	name string
}

func (f *winFile) Read(b []byte) (int, error) {
	return f.w.Read(f.name, b)
}

func (f *winFile) Write(b []byte) (int, error) {
	return f.w.Write(f.name, b)
}
