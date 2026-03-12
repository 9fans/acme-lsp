// Package text implements text editing helper routines for LSP.
package text

import (
	"fmt"
	"io"
	"os"
)

type HeadlessFile struct {
	file     *os.File
	filename string
	q0, q1   int
}

func NewHeadlessFile(filename string, q0, q1 int) (*HeadlessFile, error) {
	file, err := os.OpenFile(filename, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	return &HeadlessFile{
		file:     file,
		filename: filename,
		q0:       q0,
		q1:       q1,
	}, nil
}

// Reader returns a reader for the entire file text buffer.
func (f *HeadlessFile) Reader() (io.Reader, error) {
	if _, err := f.file.Seek(0, 0); err != nil {
		return nil, err
	}
	return f.file, nil
}

// WriteAt replaces the text in rune range [q0, q1) with bytes b.
func (f *HeadlessFile) WriteAt(q0, q1 int, b []byte) (int, error) {
	if _, err := f.file.Seek(0, 0); err != nil {
		return 0, err
	}
	body, err := io.ReadAll(f.file)
	if err != nil {
		return 0, err
	}

	// Create new body
	runes := []rune(string(body))
	if q0 < 0 || q1 > len(runes) || q0 > q1 {
		return 0, fmt.Errorf("range [%d, %d) out of bounds", q0, q1)
	}
	newRunes := append(runes[:q0], append([]rune(string(b)), runes[q1:]...)...)
	newBytes := []byte(string(newRunes))

	// Truncate and write back new body
	if err := f.file.Truncate(0); err != nil {
		return 0, err
	}
	if _, err := f.file.Seek(0, 0); err != nil {
		return 0, err
	}
	n, err := f.file.Write(newBytes)
	if err != nil {
		return 0, err
	}
	return n, f.file.Sync()
}

// Mark does nothing.
func (f *HeadlessFile) Mark() error { return nil }

// DisableMark does nothing.
func (f *HeadlessFile) DisableMark() error { return nil }

// Filename returns the filesystem path to the file.
func (f *HeadlessFile) Filename() (string, error) {
	return f.filename, nil
}

// CurrentAddr returns the address of current selection.
func (f *HeadlessFile) CurrentAddr() (q0, q1 int, err error) {
	return f.q0, f.q1, nil
}

// CloseFiles closes all the open files associated with the file.
func (f *HeadlessFile) CloseFiles() {
	f.file.Close()
}

type HeadlessMenu struct{}

func (m *HeadlessMenu) Open(filename string) (AddressableFile, error) {
	return NewHeadlessFile(filename, 0, 0)
}
