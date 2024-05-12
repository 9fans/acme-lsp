//go:build !plan9
// +build !plan9

package acme

import "9fans.net/go/plan9/client"

func mountAcme() {
	if Network == "" || Address == "" {
		panic("network or address not set")
	}
	fsys, fsysErr = client.Mount(Network, Address)
}
