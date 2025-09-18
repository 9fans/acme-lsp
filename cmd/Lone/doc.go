/*
The program Lone is a standalone client for the acme text editor that
interacts with a Language Server.

Deprecated: This program is similar to the L command, except it also does
the work of acme-lsp command by executing a LSP server on-demand. It's
recommended to use the L and acme-lsp commands instead, which takes
advantage of LSP server caches and should give faster responses.

A Language Server implements the Language Server Protocol
(see https://langserver.org/), which provides language features
like auto complete, go to definition, find all references, etc.
Lone depends on one or more language servers already being installed
in the system.  See this page of a list of language servers:
https://microsoft.github.io/language-server-protocol/implementors/servers/.

	Usage: Lone [flags] <sub-command> [args...]

List of sub-commands:

		comp
			Show auto-completion for the current cursor position.

		def
			Find where identifier at the cursor position is define and
			send the location to the plumber.

		fmt
			Organize imports and format current window buffer.

		hov
			Show more information about the identifier under the cursor
			("hover").

		monitor
			Format window buffer after each Put.

		refs
			List locations where the identifier under the cursor is used
			("references").

		rn <newname>
			Rename the identifier under the cursor to newname.

		servers
			Print list of known language servers.

		sig
			Show signature help for the function, method, etc. under
			the cursor.

		syms
			List symbols in the current file.

		assist [comp|hov|sig]
			A new window is created where completion (comp), hover
			(hov), or signature help (sig) output is shown depending
			on the cursor position in the focused window and the
			text surrounding the cursor. If the optional argument is
			given, the output will be limited to only that command.
			Note: this is a very experimental feature, and may not
			be very useful in practice.

	  -acme.addr string
	    	address where acme is serving 9P file system (default "/tmp/ns.username.:0/acme")
	  -acme.net string
	    	network where acme is serving 9P file system (default "unix")
	  -debug
	    	turn on debugging prints (deprecated: use -v)
	  -dial value
	    	map filename to language server address. The format is
	    	'handlers:host:port'. See -server flag for format of
	    	handlers. (e.g. '\.go$:localhost:4389')
	  -hidediag
	    	hide diagnostics sent by LSP server
	  -rootdir string
	    	root directory used for LSP initialization (default "/")
	  -rpc.trace
	    	print the full rpc trace in lsp inspector format
	  -server value
	    	map filename to language server command. The format is
	    	'handlers:cmd' where cmd is the LSP server command and handlers is
	    	a comma separated list of 'regexp[@lang]'. The regexp matches the
	    	filename and lang is a language identifier. (e.g. '\.go$:gopls' or
	    	'go.mod$@go.mod,go.sum$@go.sum,\.go$@go:gopls')
	  -showconfig
	    	show configuration values and exit
	  -v	Verbose output
	  -workspaces string
	    	colon-separated list of initial workspace directories
*/
package main
