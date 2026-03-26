package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			"retry": func(ts *testscript.TestScript, neg bool, args []string) {
				if len(args) == 0 {
					ts.Fatalf("retry: missing command")
				}
				timeout := 20 * time.Second
				start := time.Now()
				for {
					err := ts.Exec(args[0], args[1:]...)
					if err == nil {
						if neg {
							ts.Fatalf("retry: command %q succeeded, but expected failure", args)
						}
						return
					}
					if time.Since(start) > timeout {
						ts.Fatalf("retry: command %q failed after %v: %v", args, timeout, err)
					}
					time.Sleep(500 * time.Millisecond)
				}
			},
		},
		Setup: func(env *testscript.Env) error {
			ns := filepath.Join(env.WorkDir, "ns")
			if err := os.MkdirAll(ns, 0755); err != nil {
				return err
			}
			env.Vars = append(env.Vars, "NAMESPACE="+ns)

			for _, name := range []string{"RUSTUP_HOME", "CARGO_HOME"} {
				if val, ok := os.LookupEnv(name); ok {
					env.Vars = append(env.Vars, name+"="+val)
				} else if home, err := os.UserHomeDir(); err == nil {
					var dir string
					if name == "RUSTUP_HOME" {
						dir = filepath.Join(home, ".rustup")
					} else {
						dir = filepath.Join(home, ".cargo")
					}
					if _, err := os.Stat(dir); err == nil {
						env.Vars = append(env.Vars, name+"="+dir)
					}
				}
			}
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
