package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/fhs/acme-lsp/internal/lsp/acmelsp"
	"github.com/fhs/acme-lsp/internal/lsp/acmelsp/config"
	"github.com/fhs/acme-lsp/internal/lsp/cmd"
)

//go:generate ../../scripts/mkdocs.sh

const mainDoc = `The program acme-lsp is a client for the acme text editor that
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
https://godoc.org/github.com/fhs/acme-lsp/internal/lsp/acmelsp/config#File

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
`

func usage() {
	os.Stderr.Write([]byte(mainDoc))
	fmt.Fprintf(os.Stderr, "\n")
	flag.PrintDefaults()
	os.Exit(2)
}

var logger = log.Default()

func main() {
	flag.Usage = usage
	cfg := cmd.Setup(config.LangServerFlags | config.ProxyFlags)
	logger.SetFlags(log.Llongfile)
	logger.SetPrefix("acme-lsp: ")

	ctx := context.Background()
	app, err := NewApplication(ctx, cfg, flag.Args())
	if err != nil {
		log.Fatalf("%v", err)
	}
	err = app.Run(ctx)
	if err != nil {
		log.Fatalf("%v", err)
	}
}

type Application struct {
	cfg *config.Config
	fm  *acmelsp.FileManager
	ss  *acmelsp.ServerSet
}

func NewApplication(ctx context.Context, cfg *config.Config, args []string) (*Application, error) {
	ss, err := acmelsp.NewServerSet(cfg, acmelsp.NewDiagnosticsWriter())
	if err != nil {
		return nil, fmt.Errorf("failed to create server set: %v", err)
	}

	if len(ss.Data) == 0 {
		return nil, fmt.Errorf("no servers found in the configuration file or command line flags")
	}

	fm, err := acmelsp.NewFileManager(ss, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create file manager: %v", err)
	}
	return &Application{
		cfg: cfg,
		ss:  ss,
		fm:  fm,
	}, nil
}

func (app *Application) Run(ctx context.Context) error {
	go app.fm.Run()

	log.Print("start acmelsp ListenAndServeProxy")
	err := acmelsp.ListenAndServeProxy(ctx, app.cfg, app.ss, app.fm)
	if err != nil {
		return fmt.Errorf("proxy failed: %v", err)
	}
	return nil
}
