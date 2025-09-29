module 9fans.net/acme-lsp

go 1.21.9

replace "9fans.net/internal/go-lsp" v0.0.0-20240621142652-b2eeae9fa405 => github.com/maksym-radziwill/go-lsp-internal 18f331f33a22762d7e589394fb87f63530f724d4

require (
	9fans.net/internal/go-lsp v0.0.0-20240621142652-b2eeae9fa405
	github.com/BurntSushi/toml v0.3.1
	github.com/fhs/9fans-go v0.0.0-fhs.20200606
	github.com/google/go-cmp v0.3.0
	github.com/sourcegraph/jsonrpc2 v0.2.0
)
