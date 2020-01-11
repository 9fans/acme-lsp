The code in this directory is based on
[golang.org/x/tools/internal/lsp/protocol](https://godoc.org/golang.org/x/tools/internal/lsp/protocol)
and are subject to the license found in this directory.

We've added more flexibility in the JSON parser, which
needs to be compatible with a wider set of LSP servers, not just
[gopls](https://godoc.org/golang.org/x/tools/gopls):
* compat.go adds custom JSON unmarshaler for some types.
* Some types in tsprotocol.go have been changed to `interface{}`.
  These should have a corresponding test in compat_test.go.
