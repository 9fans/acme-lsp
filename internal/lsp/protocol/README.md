The code in this directory is based on
[golang.org/x/tools/internal/lsp/protocol](https://godoc.org/golang.org/x/tools/internal/lsp/protocol)
and are subject to the license found in this directory. We've
added more flexibility in the JSON parser (see compat.go), which
needs to be compatible with a wider set of LSP servers, not just
[gopls](https://godoc.org/golang.org/x/tools/gopls).
