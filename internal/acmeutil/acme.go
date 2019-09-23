// Copyright (c) 2009 Google Inc. All rights reserved.
// Copyright (c) 2019 Fazlul Shahriar. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Most of the code here is derived from 9fans.net/go/acme.

// Package acmeutil implements acme utility functions.
package acmeutil

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
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
	wins, err := Windows()
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

// A LogReader provides read access to the acme log file.
type LogReader struct {
	f   *client.Fid
	buf [8192]byte
}

func (r *LogReader) Close() error {
	return r.f.Close()
}

// Read reads an event from the acme log file.
func (r *LogReader) Read() (acme.LogEvent, error) {
	n, err := r.f.Read(r.buf[:])
	if err != nil {
		return acme.LogEvent{}, err
	}
	f := strings.SplitN(string(r.buf[:n]), " ", 3)
	if len(f) != 3 {
		return acme.LogEvent{}, fmt.Errorf("malformed log event")
	}
	id, _ := strconv.Atoi(f[0])
	op := f[1]
	name := f[2]
	name = strings.TrimSpace(name)
	return acme.LogEvent{
		ID:   id,
		Op:   op,
		Name: name,
	}, nil
}

// Log returns a reader reading the acme/log file.
func Log() (*LogReader, error) {
	fsys, err := getPkgFsys()
	if err != nil {
		return nil, err
	}
	f, err := fsys.Open("log", plan9.OREAD)
	if err != nil {
		return nil, err
	}
	return &LogReader{f: f}, nil
}

// Windows returns a list of the existing acme windows.
func Windows() ([]acme.WinInfo, error) {
	fsys, err := getPkgFsys()
	if err != nil {
		return nil, err
	}
	index, err := fsys.Open("index", plan9.OREAD)
	if err != nil {
		return nil, err
	}
	defer index.Close()
	data, err := ioutil.ReadAll(index)
	if err != nil {
		return nil, err
	}
	var info []acme.WinInfo
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) < 6 {
			continue
		}
		n, _ := strconv.Atoi(f[0])
		info = append(info, acme.WinInfo{ID: n, Name: f[5]})
	}
	return info, nil
}
