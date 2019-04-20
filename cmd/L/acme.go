package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"

	"9fans.net/go/acme"
	"github.com/fhs/acme-lsp/internal/lsp"
	"github.com/fhs/acme-lsp/internal/lsp/client"
	"github.com/fhs/acme-lsp/internal/lsp/text"
	"github.com/pkg/errors"
)

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

func openCurrentWin() (*win, error) {
	id, err := strconv.Atoi(os.Getenv("winid"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse $winid")
	}
	return openWin(id)
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

// CurrentAddr returns the address of current selection.
func (w *win) CurrentAddr() (q0, q1 int, err error) {
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

// Reader implements text.File.
func (w *win) Reader() (io.Reader, error) {
	_, err := w.Seek("body", 0, 0)
	if err != nil {
		return nil, errors.Wrapf(err, "seed failed for window %v", w.ID())
	}
	return w.FileReadWriter("body"), nil
}

// WriteAt implements text.File.
func (w *win) WriteAt(q0, q1 int, b []byte) (int, error) {
	err := w.Addr("#%d,#%d", q0, q1)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to write to addr for winid=%v", w.ID())
	}
	return w.Write("data", b)
}

// Mark implements text.File.
func (w *win) Mark() error {
	return w.Ctl("mark")
}

// DisableMark implements text.File.
func (w *win) DisableMark() error {
	return w.Ctl("nomark")
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

func applyAcmeEdits(we *lsp.WorkspaceEdit) error {
	wins, err := acme.Windows()
	if err != nil {
		return errors.Wrapf(err, "failed to read list of acme index")
	}
	winid := make(map[string]int, len(wins))
	for _, info := range wins {
		winid[info.Name] = info.ID
	}

	for uri := range we.Changes {
		fname := client.ToPath(uri)
		if _, ok := winid[fname]; !ok {
			return fmt.Errorf("%v: not open in acme", fname)
		}
	}
	for uri, edits := range we.Changes {
		fname := client.ToPath(uri)
		id := winid[fname]
		w, err := openWin(id)
		if err != nil {
			return errors.Wrapf(err, "failed to open window %v", id)
		}
		if err := text.Edit(w, edits); err != nil {
			return errors.Wrapf(err, "failed to apply edits to window %v", id)
		}
		w.CloseFiles()
	}
	return nil
}
