// Pakcage p9service help implement a plan9 service.
package p9service

import (
	"net"
	"os"
)

// Listen is like net.Listen but it removes dead unix sockets.
func Listen(network, address string) (net.Listener, error) {
	ln, err := net.Listen(network, address)
	if err != nil && network == "unix" && isAddrInUse(err) {
		if _, err1 := net.Dial(network, address); !isConnRefused(err1) {
			return nil, err // Listen error
		}
		// Dead socket, so remove it.
		err = os.Remove(address)
		if err != nil {
			return nil, err
		}
		return net.Listen(network, address)
	}
	return ln, err
}
