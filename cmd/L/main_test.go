package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestGetFocusedWinIDFromEnv(t *testing.T) {
	os.Setenv("winid", "123")
	defer os.Unsetenv("winid")

	want := "123"
	got, err := getFocusedWinID("")
	if err != nil {
		t.Fatalf("getFocusedWinID failed with error %v", err)
	}
	if got != want {
		t.Errorf("$winid is %v; want %v", got, want)
	}
}

func WriteAcmeFocused(ln net.Listener, winid string) error {
	conn, err := ln.Accept()
	if err != nil {
		return err
	}
	defer conn.Close()

	fmt.Fprintf(conn, "%v\n", winid)
	return nil
}

func TestGetFocusedWinIDFromServer(t *testing.T) {
	os.Unsetenv("winid")
	want := "321"

	dir, err := os.MkdirTemp("", "acmefocused")
	if err != nil {
		t.Fatalf("couldn't create temporary directory: %v", err)
	}
	defer os.RemoveAll(dir)
	addr := filepath.Join(dir, "acmefocused")

	ln, err := net.Listen("unix", addr)
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	go func() {
		err := WriteAcmeFocused(ln, want)
		if err != nil {
			t.Errorf("acmefocused server failed: %v", err)
		}
	}()

	got, err := getFocusedWinID(addr)
	if err != nil {
		t.Fatalf("getFocusedWinID failed with error %v", err)
	}
	if got != want {
		t.Errorf("$winid is %v; want %v", got, want)
	}
}
