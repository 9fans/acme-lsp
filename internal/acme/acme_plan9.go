// +build plan9

package acme

import (
	"io"
	"os"

	"golang.org/x/sys/plan9"
)

// On Plan 9, the acme file system is already mounted at /mnt/acme if acme is running.
func mountAcme() {
	_, fsysErr = os.Stat("/mnt/acme")
}

// p9fd implements clientFid.
type p9fd int

func (fd p9fd) Read(b []byte) (n int, err error) {
	n, err = plan9.Read(int(fd), b)
	if n == 0 && err == nil {
		err = io.EOF
	}
	return
}

func (fd p9fd) ReadAt(b []byte, off int64) (n int, err error) {
	n, err = plan9.Pread(int(fd), b, off)
	if n == 0 && err == nil {
		err = io.EOF
	}
	return
}

func (fd p9fd) Write(b []byte) (n int, err error) {
	return plan9.Write(int(fd), b)
}

func (fd p9fd) Seek(off int64, whence int) (newoff int64, err error) {
	return plan9.Seek(int(fd), off, whence)
}

func (fd p9fd) Close() error {
	return plan9.Close(int(fd))
}

func acmefsOpen(name string, mode uint8) (p9fd, error) {
	fd, err := plan9.Open("/mnt/acme/"+name, int(mode))
	return p9fd(fd), err
}
