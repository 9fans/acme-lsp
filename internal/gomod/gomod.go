package gomod

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
)

type env struct {
	GOPATH, GOROOT string
}

func getEnv() (*env, error) {
	out, err := exec.Command("go", "env", "-json").Output()
	if err != nil {
		return nil, err
	}
	var e env
	err = json.Unmarshal(out, &e)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

type module struct {
	Path string
}

func getModulePath(dir string) (string, error) {
	cmd := exec.Command("go", "list", "-m", "-json")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	var m module
	err = json.Unmarshal(out, &m)
	if err != nil {
		return "", err
	}
	return m.Path, nil
}

func isSubdirectory(parent, child string) bool {
	p := filepath.Clean(parent)
	c := filepath.Clean(child)
	return (p == c) || (len(c) > len(p) && strings.HasPrefix(c, p) && c[len(p)] == filepath.Separator)
}

// RootDir returns the module root directory for filename.
func RootDir(filename string) string {
	defaultRoot := "/" // TODO(fhs): windows support?

	e, err := getEnv()
	if err != nil {
		return defaultRoot
	}
	dir := filepath.Dir(filename)
	if isSubdirectory(e.GOPATH, dir) || isSubdirectory(e.GOROOT, dir) {
		return defaultRoot
	}
	r, err := getModulePath(dir)
	if err != nil {
		return defaultRoot
	}
	return r
}
