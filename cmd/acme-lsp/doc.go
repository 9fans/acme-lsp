/*
The program acme-lsp is a client for the acme text editor that
acts as a proxy for a set of Language Server Protocol servers.

A Language Server implements the Language Server Protocol
(see https://langserver.org/), which provides language features
like auto complete, go to definition, find all references, etc.
Acme-lsp depends on one or more language servers already being
installed in the system.  See this page of a list of language servers:
https://microsoft.github.io/language-server-protocol/implementors/servers/.

Acme-lsp is optionally configured using a TOML-based configuration file
located at UserConfigDir/acme-lsp/config.toml (the -showconfig flag
prints the exact location).  The command line flags will override the
configuration values.  The configuration options are described here:
https://godoc.org/9fans.net/acme-lsp/internal/lsp/acmelsp/config#File

Acme-lsp executes or connects to a set of LSP servers described in the
configuration file or in the -server or -dial flags. It then listens for
messages sent by the L command, which direct acme-lsp to run commands
on the LSP servers and apply/show the results in acme. The communication
protocol used here is an implementation detail that is subject to change.

Acme-lsp watches for files created (New), loaded (Get), saved (Put), or
deleted (Del) in acme, and tells the LSP server about these changes. The
LSP server in turn responds by sending diagnostics information (compiler
errors, lint errors, etc.) which are shown in a "/LSP/Diagnostics" window.
Also, when Put is executed in an acme window, acme-lsp will organize
import paths in the window and format it by default. This behavior can
be changed by the FormatOnPut and CodeActionsOnPut configuration options.

		Usage: acme-lsp [flags]

	  -acme.addr string
	    	address where acme is serving 9P file system (default "/tmp/ns.fhs.:0/acme")
	  -acme.net string
	    	network where acme is serving 9P file system (default "unix")
	  -debug
	    	turn on debugging prints (deprecated: use -v)
	  -dial value
	    	language server address for filename match (e.g. '\.go$:localhost:4389')
	  -proxy.addr string
	    	address used for communication between acme-lsp and L (default "/tmp/ns.fhs.:0/acme-lsp.rpc")
	  -proxy.net string
	    	network used for communication between acme-lsp and L (default "unix")
	  -rootdir string
	    	root directory used for LSP initialization. (default "/")
	  -server value
	    	language server command for filename match (e.g. '\.go$:gopls')
	  -showconfig
	    	show configuration values and exit
	  -v	Verbose output
	  -workspaces string
	    	colon-separated list of initial workspace directories
*/
package main
