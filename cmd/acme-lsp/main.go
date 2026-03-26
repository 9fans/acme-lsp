package main

import "9fans.net/acme-lsp/internal/lsp/cmd/acmelsp"

//go:generate ../../scripts/mkdocs.sh

func main() {
	acmelsp.Main()
}
