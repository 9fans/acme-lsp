//go:build plan9
// +build plan9

package p9service

func isAddrInUse(err error) bool {
	return false
}

func isConnRefused(err error) bool {
	return false
}
