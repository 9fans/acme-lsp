// Package acmeutil implements acme utility functions.
package acmeutil

import (
	"bytes"
	"io"
	"os"
	"strconv"

	"9fans.net/go/acme"
	"github.com/pkg/errors"
)

type Win struct {
	*acme.Win
}

func NewWin() (*Win, error) {
	w, err := acme.New()
	if err != nil {
		return nil, err
	}
	return &Win{w}, err
}

func OpenWin(id int) (*Win, error) {
	w, err := acme.Open(id, nil)
	if err != nil {
		return nil, err
	}
	return &Win{w}, err
}

func OpenCurrentWin() (*Win, error) {
	id, err := strconv.Atoi(os.Getenv("winid"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse $winid")
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
		return nil, errors.Wrapf(err, "seed failed for window %v", w.ID())
	}
	return w.FileReadWriter("body"), nil
}

// WriteAt implements text.File.
func (w *Win) WriteAt(q0, q1 int, b []byte) (int, error) {
	err := w.Addr("#%d,#%d", q0, q1)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to write to addr for winid=%v", w.ID())
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
