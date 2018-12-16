[![Build Status](https://travis-ci.com/fhs/acme-lsp.svg?branch=master)](https://travis-ci.com/fhs/acme-lsp)
[![GoDoc](https://godoc.org/github.com/fhs/acme-lsp/cmd/L?status.svg)](https://godoc.org/github.com/fhs/acme-lsp/cmd/L)
[![Go Report Card](https://goreportcard.com/badge/github.com/fhs/acme-lsp)](https://goreportcard.com/report/github.com/fhs/acme-lsp)

# acme-lsp

[Language Server Protocol](https://langserver.org/) tools for [acme](https://en.wikipedia.org/wiki/Acme_(text_editor)).

Documentation: https://godoc.org/github.com/fhs/acme-lsp/cmd/L

## Status

Basic commands are working with [go-langserver](https://github.com/sourcegraph/go-langserver) and [pyls](https://github.com/palantir/python-language-server). Other language servers need to be tested.

## See also

* https://github.com/davidrjenni/A - Similar tool but only for Go programming language
* https://godoc.org/9fans.net/go/acme/acmego - Similar to `L monitor` sub-command but only for Go
* https://godoc.org/github.com/fhs/misc/cmd/acmepy - Python formatter based on acmego
