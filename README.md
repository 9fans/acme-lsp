[![Build Status](https://travis-ci.com/fhs/acme-lsp.svg?branch=master)](https://travis-ci.com/fhs/acme-lsp)
[![GoDoc](https://godoc.org/github.com/fhs/acme-lsp/cmd/acme-lsp?status.svg)](https://godoc.org/github.com/fhs/acme-lsp/cmd/acme-lsp)
[![Go Report Card](https://goreportcard.com/badge/github.com/fhs/acme-lsp)](https://goreportcard.com/report/github.com/fhs/acme-lsp)

# acme-lsp

[Language Server Protocol](https://langserver.org/) tools for [acme](https://en.wikipedia.org/wiki/Acme_(text_editor)) text editor.

The main tool is
[acme-lsp](https://godoc.org/github.com/fhs/acme-lsp/cmd/acme-lsp),
which listens for commands from the [L
command](https://godoc.org/github.com/fhs/acme-lsp/cmd/L).
It also watches for files created (`New`), loaded (`Get`), saved
(`Put`), or deleted (`Del`) in acme, and tells the LSP server about
these changes. The LSP server in turn responds by sending diagnostics
information (compiler errors, lint errors, etc.) which are shown in an
acme window.  When `Put` is executed in an acme window, `acme-lsp`
also organizes import paths in the window and formats it.

Currently, `acme-lsp` has been tested with
[gopls](https://godoc.org/golang.org/x/tools/cmd/gopls),
[go-langserver](https://github.com/sourcegraph/go-langserver) and
[pyls](https://github.com/palantir/python-language-server). Please report
incompatibilities with those or other servers.

## Installation

	go get -u github.com/fhs/acme-lsp/cmd/acme-lsp
	go get -u github.com/fhs/acme-lsp/cmd/L

## gopls

First install the latest release of gopls:

	GO111MODULE=on go get golang.org/x/tools/gopls@latest

Start acme-lsp like this:

	acme-lsp -server '\.go$:gopls' -workspaces /path/to/mod1:/path/to/mod2

where mod1 and mod2 are module directories with a `go.mod` file.
The set of workspace directories can be changed at runtime
by using the `L ws+` and `L ws-` sub-commands.

When `Put` is executed in an acme window editing `.go` file, acme-lsp
will update import paths and gofmt the window buffer if needed.  It also
enables commands like `L def` (jump to defenition), `L refs` (list of
references), etc. within acme. Note: any output from these commands are
printed to stdout by `acme-lsp`, so it's beneficial to start `acme-lsp` from
within acme, where the output is written to `+Errors` window.

## Hints & Tips

* If a file gets out of sync in the LSP server (e.g. because you edited
the file outside of acme), executing `Get` on the file will update it
in the LSP server.

* Create scripts like `Ldef`, `Lrefs`, `Ltype`, etc., so that you can
easily execute those commands with a single middle click:
```
for cmd in comp def fmt hov refs rn sig syms type assist ws ws+ ws-
do
	cat > L${cmd} <<EOF
#!/bin/sh
exec L ${cmd} "\$@"
EOF
	chmod +x L${cmd}
done
```

* Create custom keybindings that allow you to do completion
(`L comp -e`) and show signature help (`L sig`) while you're
typing. This can be achieved by using a general keybinding daemon
(e.g. [xbindkeys](http://www.nongnu.org/xbindkeys/xbindkeys.html)
in X11) and running
[acmefocused](https://godoc.org/github.com/fhs/acme-lsp/cmd/acmefocused).

## See also

* https://github.com/davidrjenni/A - Similar tool but only for Go programming language
* https://godoc.org/9fans.net/go/acme/acmego - Implements formatting and import fixes for Go
* https://godoc.org/github.com/fhs/misc/cmd/acmepy - Python formatter based on acmego
