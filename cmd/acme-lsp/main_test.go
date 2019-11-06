package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/fhs/acme-lsp/internal/acme"
	"github.com/fhs/acme-lsp/internal/lsp/acmelsp/config"
)

func TestMain(m *testing.M) {
	// TODO: Replace Xvfb with a fake devdraw.
	var x *exec.Cmd
	switch runtime.GOOS {
	case "linux", "freebsd", "openbsd", "netbsd", "dragonfly":
		if os.Getenv("DISPLAY") == "" {
			dp := fmt.Sprintf(":%d", xvfbServerNumber())
			x = exec.Command("Xvfb", dp, "-screen", "0", "1024x768x24")
			x.Stdout = os.Stdout
			x.Stderr = os.Stderr
			if err := x.Start(); err != nil {
				log.Fatalf("failed to execute Xvfb: %v", err)
			}
			// Wait for Xvfb to start up.
			for i := 0; i < 5*60; i++ {
				err := exec.Command("xdpyinfo", "-display", dp).Run()
				if err == nil {
					break
				}
				if _, ok := err.(*exec.ExitError); !ok {
					log.Fatalf("failed to execute xdpyinfo: %v", err)
				}
				log.Printf("waiting for Xvfb (try %v) ...\n", i)
				time.Sleep(time.Second)
			}
			os.Setenv("DISPLAY", dp)
		}
	}
	e := m.Run()

	if x != nil {
		// Kill Xvfb gracefully, so that it cleans up the /tmp/.X*-lock file.
		x.Process.Signal(os.Interrupt)
		x.Wait()
	}
	os.Exit(e)
}

// XvfbServerNumber finds a free server number for Xfvb.
// Similar logic is used by /usr/bin/xvfb-run:/^find_free_servernum/
func xvfbServerNumber() int {
	for n := 99; n < 1000; n++ {
		if _, err := os.Stat(fmt.Sprintf("/tmp/.X%d-lock", n)); os.IsNotExist(err) {
			return n
		}
	}
	panic("no free X server number")
}

func TestAcmeLSP(t *testing.T) {
	dir, err := ioutil.TempDir("", "acme-lsp-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	os.Setenv("NAMESPACE", dir)

	cfg := config.Default()
	cfg.Servers = map[string]*config.Server{
		"gopls": {
			Command: []string{"gopls", "serve"},
		},
	}
	cfg.FilenameHandlers = []config.FilenameHandler{
		{
			Pattern:   `\.go`,
			ServerKey: "gopls",
		},
	}
	// Setup custom acme package
	acme.Network = cfg.AcmeNetwork
	acme.Address = cfg.AcmeAddress

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	defer func() {
		cancel()
		wg.Wait()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		// Use Edwood instead of plan9port acme because it's
		// easier to install on CI systems and it seems to be
		// easier to kill than acme. Sending a KILL signal to
		// acme doesn't really kill it.
		cmd := exec.CommandContext(ctx, "edwood", "-c", "1")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("NAMESPACE=%v", dir))
		if err := cmd.Run(); err != nil {
			t.Logf("edwood died: %v", err)
		}
	}()

	// Wait for acme to start up.
	acmeReady := false
	for i := 0; i < 5*60; i++ {
		conn, err := net.Dial(cfg.AcmeNetwork, cfg.AcmeAddress)
		if err == nil {
			conn.Close()
			acmeReady = true
			break
		}
		t.Logf("waiting for acme (try %d) ...", i)
		time.Sleep(time.Second)
	}
	if !acmeReady {
		t.Fatalf("acme did not become ready")
	}

	app, err := NewApplication(ctx, cfg, nil)
	if err != nil {
		t.Fatalf("failed to create application: %v", err)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := app.Run(ctx); err != nil {
			t.Logf("acme-lsp: %v", err)
		}
	}()
}
