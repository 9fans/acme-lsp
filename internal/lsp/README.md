The code in this directory is based on
[sourcegraph/go-lsp](github.com/sourcegraph/go-lsp). We add more types and more
flexibility in the JSON parser, which needs to be compatible with a wider set
of LSP servers, not just
[go-langserver](github.com/sourcegraph/go-langserver).

# go-lsp

Package lsp contains Go types for the messages used in the Language Server
Protocol.

See
https://github.com/Microsoft/language-server-protocol/blob/master/protocol.md
for more information.
