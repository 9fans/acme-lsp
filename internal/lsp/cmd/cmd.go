// Pakcage cmd contains utlity functions that help implement lsp related commands.
package cmd

import (
	"flag"
	"log"
	"os"

	"github.com/fhs/acme-lsp/internal/acme"
	"github.com/fhs/acme-lsp/internal/lsp/acmelsp"
	"github.com/fhs/acme-lsp/internal/lsp/acmelsp/config"
)

func Setup(flags config.Flags) *config.Config {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config file: %v", err)
	}
	err = cfg.ParseFlags(flags, flag.CommandLine, os.Args[1:])
	if err != nil {
		// Unreached since flag.CommandLine uses flag.ExitOnError.
		log.Fatalf("failed to parse flags: %v", err)
	}

	if cfg.ShowConfig {
		config.Write(os.Stdout, cfg)
		os.Exit(0)
	}

	// Setup custom acme package
	acme.Network = cfg.AcmeNetwork
	acme.Address = cfg.AcmeAddress

	if cfg.Verbose {
		acmelsp.Verbose = true
	}
	return cfg
}
