// +build !plan9

package acmelsp

import (
	"io"

	"9fans.net/go/plan9"
	"9fans.net/go/plumb"
)

func plumbOpenSend() (io.WriteCloser, error) {
	return plumb.Open("send", plan9.OWRITE)
}
