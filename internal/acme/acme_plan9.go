package acme

import (
	"9fans.net/go/plan9/client"
)

func mountAcme() {
	// Already mounted at /mnt/acme.
	// Ignore Network and Address.
	fsys = &client.Fsys{Mtpt: "/mnt/acme"}
	fsysErr = nil
}
