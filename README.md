[![GitHub Actions Status](https://github.com/9fans/acme-lsp/workflows/Test/badge.svg)](https://github.com/9fans/acme-lsp/actions?query=branch%3Amaster+event%3Apush)
[![Go Reference](https://pkg.go.dev/badge/9fans.net/acme-lsp/cmd/acme-lsp.svg)](https://pkg.go.dev/9fans.net/acme-lsp/cmd/acme-lsp)
[![Go Report Card](https://goreportcard.com/badge/9fans.net/acme-lsp)](https://goreportcard.com/report/9fans.net/acme-lsp)

# acme-lsp

[Language Server Protocol](https://langserver.org/) tools for [acme](https://en.wikipedia.org/wiki/Acme_(text_editor)) text editor.

The main tool is
[acme-lsp](https://pkg.go.dev/9fans.net/acme-lsp/cmd/acme-lsp),
which listens for commands from the [L
command](https://pkg.go.dev/9fans.net/acme-lsp/cmd/L).
It also watches for files created (`New`), loaded (`Get`), saved
(`Put`), or deleted (`Del`) in acme, and tells the LSP server about
these changes. The LSP server in turn responds by sending diagnostics
information (compiler errors, lint errors, etc.) which are shown in an
acme window.  When `Put` is executed in an acme window, `acme-lsp`
also organizes import paths in the window and formats it.

Currently, `acme-lsp` has been tested with
[gopls](https://github.com/golang/tools/tree/master/gopls),
[go-langserver](https://github.com/sourcegraph/go-langserver) and
[pyls](https://github.com/palantir/python-language-server). Please report
incompatibilities with those or other servers.

## Installation

Install the latest release:

	GO111MODULE=on go install 9fans.net/acme-lsp/cmd/acme-lsp@latest
	GO111MODULE=on go install 9fans.net/acme-lsp/cmd/L@latest

## gopls

First install the latest release of gopls:

	GO111MODULE=on go install golang.org/x/tools/gopls@latest

Start acme-lsp like this:

	acme-lsp -server '([/\\]go\.mod)|([/\\]go\.sum)|(\.go)$:gopls serve' -workspaces /path/to/mod1:/path/to/mod2

where mod1 and mod2 are module directories with a `go.mod` file.
The set of workspace directories can be changed at runtime
by using the `L ws+` and `L ws-` sub-commands.

When `Put` is executed in an acme window editing `.go` file, acme-lsp
will update import paths and gofmt the window buffer if needed.  It also
enables commands like `L def` (jump to defenition), `L refs` (list of
references), etc. within acme. The `L assist` command opens a window
where completion, hover, or signature help output is shown for the
current cursor position in the `.go` file being edited.

If you want to change `gopls`
[settings](https://github.com/golang/tools/blob/master/gopls/doc/settings.md),
you can create a configuration file at
`UserConfigDir/acme-lsp/config.toml` (the `-showconfig` flag prints
the exact location) and then run `acme-lsp` without any flags. Example
config file:
```toml
WorkspaceDirectories = [
	"/path/to/mod1",
	"/path/to/mod2",
]
FormatOnPut = true
CodeActionsOnPut = ["source.organizeImports"]

[Servers]
	[Servers.gopls]
	Command = ["gopls", "serve", "-rpc.trace"]
	StderrFile = "gopls.stderr.log"
	LogFile = "gopls.log"

		# These settings gets passed to gopls
		[Servers.gopls.Options]
		hoverKind = "FullDocumentation"

[[FilenameHandlers]]
  Pattern = "[/\\\\]go\\.mod$"
  LanguageID = "go.mod"
  ServerKey = "gopls"

[[FilenameHandlers]]
  Pattern = "[/\\\\]go\\.sum$"
  LanguageID = "go.sum"
  ServerKey = "gopls"

[[FilenameHandlers]]
  Pattern = "\\.go$"
  # Don't run this LSP in files in /projects/example (for instance because that
  # project needs a gopls with a Bazel packages driver).
  Ignore = "/projects/example"
  LanguageID = "go"
  ServerKey = "gopls"
```

## Hints & Tips

* If a file gets out of sync in the LSP server (e.g. because you edited
the file outside of acme), executing `Get` on the file will update it
in the LSP server.

* Create scripts like `Ldef`, `Lrefs`, `Ltype`, etc., so that you can
easily execute those commands with a single middle click:
```
for(cmd in comp def fmt hov impls refs rn sig syms type assist ws ws+ ws-){
	> L^$cmd {
		echo '#!/bin/rc'
		echo exec L $cmd '$*'
	}
	chmod +x L^$cmd
}
```

* Create custom keybindings that allow you to do completion
(`L comp -e`) and show signature help (`L sig`) while you're
typing. This can be achieved by using a general keybinding daemon
(e.g. [xbindkeys](http://www.nongnu.org/xbindkeys/xbindkeys.html)
in X11) and running
[acmefocused](https://pkg.go.dev/9fans.net/acme-lsp/cmd/acmefocused).

## See also

* [A setup with Acme on Darwin using acme-lsp with ccls](https://www.bytelabs.org/posts/acme-lsp/) by Igor Böhm
* https://github.com/davidrjenni/A - Similar tool but only for Go programming language
* https://pkg.go.dev/9fans.net/go/acme/acmego - Implements formatting and import fixes for Go
* https://pkg.go.dev/github.com/fhs/misc/cmd/acmepy - Python formatter based on acmego
* https://github.com/ilanpillemer/acmecrystal - Crystal formatter
