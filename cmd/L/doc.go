/*
The program L sends messages to the Language Server Protocol
proxy server acme-lsp.

L is usually run from within the acme text editor, where $winid
environment variable is set to the ID of currently focused window.
It sends this ID to acme-lsp, which uses it to compute the context for
LSP commands.

If L is run outside of acme (therefore $winid is not set), L will
attempt to find the focused window ID by connecting to acmefocused
(https://godoc.org/github.com/fhs/acme-lsp/cmd/acmefocused).

	Usage: L <sub-command> [args...]

List of sub-commands:

		comp [-e]
			Print candidate completions at the cursor position. If
			-e (edit) flag is given and there is only one candidate,
			the completion is applied instead of being printed.

		def [-p]
			Find where the symbol at the cursor position is defined
			and send the location to the plumber. If -p flag is given,
			the location is printed to stdout instead.

		fmt
			Organize imports and format current window buffer.

		hov
			Show more information about the symbol under the cursor
			("hover").

		impls
			List implementation location(s) of the symbol under the cursor.

		refs
			List locations where the symbol under the cursor is used
			("references").

		rn <newname>
			Rename the symbol under the cursor to newname.

		sig
			Show signature help for the function, method, etc. under
			the cursor.

		syms
			List symbols in the current file.

		type [-p]
			Find where the type of the symbol at the cursor position
			is defined and send the location to the plumber. If -p
			flag is given, the location is printed to stdout instead.

		assist [comp|hov|sig]
			A new window is created where completion (comp), hover
			(hov), or signature help (sig) output is shown depending
			on the cursor position in the focused window and the
			text surrounding the cursor. If the optional argument is
			given, the output will be limited to only that command.
			Note: this is a very experimental feature, and may not
			be very useful in practice.

		ws
			List current set of workspace directories.

		ws+ [directories...]
			Add given directories to the set of workspace directories.
			Current working directory is added if no directory is specified.

		ws- [directories...]
			Remove given directories to the set of workspace directories.
			Current working directory is removed if no directory is specified.

	  -acme.addr string
	    	address where acme is serving 9P file system (default "/tmp/ns.fhs.:0/acme")
	  -acme.net string
	    	network where acme is serving 9P file system (default "unix")
	  -proxy.addr string
	    	address used for communication between acme-lsp and L (default "/tmp/ns.fhs.:0/acme-lsp.rpc")
	  -proxy.net string
	    	network used for communication between acme-lsp and L (default "unix")
	  -showconfig
	    	show configuration values and exit
	  -v	Verbose output
*/
package main
