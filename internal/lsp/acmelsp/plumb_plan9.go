// +build plan9

package acmelsp

import (
	"io"

	"golang.org/x/sys/plan9"
)

// p9fd implements io.WriteCloser.
type p9fd int

func (fd p9fd) Write(b []byte) (n int, err error) {
	return plan9.Write(int(fd), b)
}

func (fd p9fd) Close() error {
	return plan9.Close(int(fd))
}

// Cf. /sys/src/libplumb/mesg.c:/^plumbopen/
func plumbOpenSend() (io.WriteCloser, error) {
	fd, err := plan9.Open("/mnt/plumb/send", plan9.O_WRONLY)
	return p9fd(fd), err
}
