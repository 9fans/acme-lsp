// Package acmeutil implements acme utility functions.
package acmeutil

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

	"9fans.net/go/acme"
	"9fans.net/go/plan9"
	"9fans.net/go/plan9/client"
	"github.com/pkg/errors"
)

var (
	pkgFsys    *client.Fsys
	pkgFsysErr = errors.New("not mounted")
	pkgFsysMu  sync.Mutex
)

// Mount mounts acme file system. It should be called before using this
// package. This setup step is needed to support custom network address
// (e.g. in Windows), and since 9fans.net/go/plan9/client doesn't provide
// a way to unmount the file system, we want to reuse one connection in
// order to avoid file descriptor leaks.
func Mount(network, addr string) error {
	pkgFsysMu.Lock()
	defer pkgFsysMu.Unlock()

	if pkgFsysErr == nil {
		return errors.New("already mounted")
	}
	pkgFsys, pkgFsysErr = client.Mount(network, addr)
	return pkgFsysErr
}

func getPkgFsys() (*client.Fsys, error) {
	pkgFsysMu.Lock()
	defer pkgFsysMu.Unlock()

	return pkgFsys, pkgFsysErr
}

type Win struct {
	*acme.Win
}

func NewWin() (*Win, error) {
	fsys, err := getPkgFsys()
	if err != nil {
		return nil, err
	}
	fid, err := fsys.Open("new/ctl", plan9.ORDWR)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 100)
	n, err := fid.Read(buf)
	if err != nil {
		fid.Close()
		return nil, err
	}
	a := strings.Fields(string(buf[0:n]))
	if len(a) == 0 {
		fid.Close()
		return nil, errors.New("short read from acme/new/ctl")
	}
	id, err := strconv.Atoi(a[0])
	if err != nil {
		fid.Close()
		return nil, errors.New("invalid window id in acme/new/ctl: " + a[0])
	}
	w, err := acme.Open(id, fid)
	if err != nil {
		return nil, err
	}
	return &Win{w}, nil
}

func OpenWin(id int) (*Win, error) {
	fsys, err := getPkgFsys()
	if err != nil {
		return nil, err
	}
	ctl, err := fsys.Open(fmt.Sprintf("%d/ctl", id), plan9.ORDWR)
	if err != nil {
		return nil, err
	}
	w, err := acme.Open(id, ctl)
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
