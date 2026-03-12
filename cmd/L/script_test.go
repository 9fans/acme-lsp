package main

import (
	"os"
	"testing"

	acmelsp "9fans.net/acme-lsp/internal/lsp/cmd/acmelsp"
	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"acme-lsp": acmelspCmd,
		"L":        LCmd,
	}))
}

func TestL(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata",
		Setup: func(env *testscript.Env) error {
			return nil
		},
	})
}

func acmelspCmd() int {
	acmelsp.Main()
	return 0
}

func LCmd() int {
	main()
	return 0
}
