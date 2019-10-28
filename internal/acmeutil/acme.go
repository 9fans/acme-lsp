// Package acmeutil implements acme utility functions.
package acmeutil

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/fhs/acme-lsp/internal/acme"
)

type Win struct {
	*acme.Win
}

func NewWin() (*Win, error) {
	w, err := acme.New()
	if err != nil {
		return nil, err
	}
	return &Win{w}, nil
}

func OpenWin(id int) (*Win, error) {
	w, err := acme.Open(id, nil)
	if err != nil {
		return nil, err
	}
	return &Win{w}, nil
}

func OpenCurrentWin() (*Win, error) {
	id, err := strconv.Atoi(os.Getenv("winid"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse $winid: %v", err)
	}
	return OpenWin(id)
}

func (w *Win) Filename() (string, error) {
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
func (w *Win) CurrentAddr() (q0, q1 int, err error) {
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

func (w *Win) FileReadWriter(filename string) io.ReadWriter {
	return &winReadWriter{
		w:    w.Win,
		name: filename,
	}
}

// Reader implements text.File.
func (w *Win) Reader() (io.Reader, error) {
	_, err := w.Seek("body", 0, 0)
	if err != nil {
		return nil, fmt.Errorf("seed failed for window %v: %v", w.ID(), err)
	}
	return w.FileReadWriter("body"), nil
}

// WriteAt implements text.File.
func (w *Win) WriteAt(q0, q1 int, b []byte) (int, error) {
	err := w.Addr("#%d,#%d", q0, q1)
	if err != nil {
		return 0, fmt.Errorf("failed to write to addr for winid=%v: %v", w.ID(), err)
	}
	return w.Write("data", b)
}

// Mark implements text.File.
func (w *Win) Mark() error {
	return w.Ctl("mark")
}

// DisableMark implements text.File.
func (w *Win) DisableMark() error {
	return w.Ctl("nomark")
}

type winReadWriter struct {
	w    *acme.Win
	name string
}

func (f *winReadWriter) Read(b []byte) (int, error) {
	return f.w.Read(f.name, b)
}

func (f *winReadWriter) Write(b []byte) (int, error) {
	return f.w.Write(f.name, b)
}

// Hijack returns the first window named name
// found in the set of existing acme windows.
func Hijack(name string) (*Win, error) {
	wins, err := acme.Windows()
	if err != nil {
		return nil, fmt.Errorf("hijack %q: %v", name, err)
	}
	for _, info := range wins {
		if info.Name == name {
			return OpenWin(info.ID)
		}
	}
	return nil, fmt.Errorf("hijack %q: window not found", name)
}
