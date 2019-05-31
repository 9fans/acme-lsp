[![Build Status](https://travis-ci.com/fhs/acme-lsp.svg?branch=master)](https://travis-ci.com/fhs/acme-lsp)
[![GoDoc](https://godoc.org/github.com/fhs/acme-lsp/cmd/acme-lsp?status.svg)](https://godoc.org/github.com/fhs/acme-lsp/cmd/acme-lsp)
[![Go Report Card](https://goreportcard.com/badge/github.com/fhs/acme-lsp)](https://goreportcard.com/report/github.com/fhs/acme-lsp)

# acme-lsp

[Language Server Protocol](https://langserver.org/) tools for [acme](https://en.wikipedia.org/wiki/Acme_(text_editor)) text editor.

The main tool is
[acme-lsp](https://godoc.org/github.com/fhs/acme-lsp/cmd/acme-lsp),
which listens on the plumber port `lsp` for commands from the [L
command](https://godoc.org/github.com/fhs/acme-lsp/cmd/L). It also
watches for `Put` executed in an acme window, organizes import paths in
the window and formats it.

Currently, acme-lsp has been tested with
[gopls](https://godoc.org/golang.org/x/tools/cmd/gopls),
[go-langserver](https://github.com/sourcegraph/go-langserver) and
[pyls](https://github.com/palantir/python-language-server). Please report
incompatibilities with those or other servers.

## Installation

	go get -u github.com/fhs/acme-lsp/cmd/acme-lsp
	go get -u github.com/fhs/acme-lsp/cmd/L

## gopls

First install gopls:

	go get -u golang.org/x/tools/cmd/gopls

Add an empty plumbing rule to $HOME/lib/plumbing for acme-lsp:

	# declarations of ports without rules
	plumb to lsp

Make sure plumber is running and reload the rules by running:

	cat $HOME/lib/plumbing | 9p write plumb/rules

Start acme-lsp like this:

	acme-lsp -server '\.go$:gopls' -workspaces /path/to/mod1:/path/to/mod2

where mod1 and mod2 are module directories with a `go.mod` file.
The set of workspace directories can be changed at runtime
by using the `L ws+` and `L ws-` sub-commands.

When `Put` is executed in an acme window editing `.go` file, acme-lsp
will update import paths and gofmt the window buffer if needed.  It also
enables commands like `L def` (jump to defenition), `L sig` (signature
help), etc. within acme.

## See also

* https://github.com/davidrjenni/A - Similar tool but only for Go programming language
* https://godoc.org/9fans.net/go/acme/acmego - Similar to `L monitor` sub-command but only for Go
* https://godoc.org/github.com/fhs/misc/cmd/acmepy - Python formatter based on acmego
