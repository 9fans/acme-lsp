/*
The program acme-lsp is a client for the acme text editor that
acts as a proxy for a set of Language Server Protocol servers.

A Language Server implements the Language Server Protocol
(see https://langserver.org/), which provides language features
like auto complete, go to definition, find all references, etc.
Acme-lsp depends on one or more language servers already being
installed in the system.  See this page of a list of language servers:
https://microsoft.github.io/language-server-protocol/implementors/servers/.

Acme-lsp executes or connects to a set of LSP servers specified using the
-server or -dial flags.  It then listens for messages sent to the plumber
(https://9fans.github.io/plan9port/man/man4/plumber.html) port named
"lsp".  The messages direct acme-lsp to run commands on the LSP servers
and apply/show the results in acme.  The format of the plumbing messages
is implementation dependent and subject to change. The L command should
be used to send the messages instead of plumb(1) command.

This following plumbing rule must be added to $HOME/lib/plumbing:

	# declarations of ports without rules
	plumb to lsp

and then the rules must be reload by running:

	cat $HOME/lib/plumbing | 9p write plumb/rules

Acme-lsp also watches for Put executed in an acme window, organizes
import paths in the window and formats it.

	Usage: acme-lsp [flags]

  -debug
    	turn on debugging prints
  -dial value
    	language server address for filename match (e.g. '\.go$:localhost:4389')
  -server value
    	language server command for filename match (e.g. '\.go$:gopls')
  -workspaces string
    	colon-separated list of initial workspace directories
*/
package main
