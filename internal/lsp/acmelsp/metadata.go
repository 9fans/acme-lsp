// Package acmelsp implements the core of acme-lsp commands.
package acmelsp

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/fhs/acme-lsp/internal/lsp/protocol"
)

//metdata is implemented due to the https://github.com/OmniSharp/omnisharp-roslyn/issues/2238

func convertFilePath(p string) (path string) {
	path = strings.Replace(p, "$metadata$", fmt.Sprintf("%s/csharp-metadata/", os.TempDir()), 1) //os.TempDir
	return
}

func GetMetaParas(s string) (*protocol.MetadataParams, bool) { //
	out := &protocol.MetadataParams{TimeOut: 5000}
	parts := strings.Split(s, "/Assembly/")

	if len(parts) < 2 {
		return nil, false
	}

	pName := strings.TrimPrefix(parts[0], "file:///%24metadata%24/Project/")

	out.ProjectName = pName

	parts = strings.Split(parts[1], "/Symbol/")

	if len(parts) != 2 {
		return nil, false
	}

	assemblyName := strings.Replace(parts[0], "/", ".", -1)

	out.AssemblyName = assemblyName

	typeName := strings.Replace(strings.TrimSuffix(parts[1], ".cs"), "/", ".", -1)

	out.TypeName = typeName

	return out, true

}

func (rc *RemoteCmd) localizeMetadata(ctx context.Context, uri string) (string, error) {
	p, ok := GetMetaParas(uri)
	if !ok {
		return "", fmt.Errorf("failed to parse URI to MetaParas")
	}
	// server over here is the acme-lsp server
	src, err := rc.server.Metadata(ctx, p)

	if err != nil {
		return "", fmt.Errorf("failed to retrive metatdata of uri: %s, with err: %w", uri, err)
	}

	key := src.SourceName

	if v, ok := rc.metadataSet[key]; ok {
		return v, nil
	}

	path := convertFilePath(key)

	if err := os.MkdirAll(filepath.Dir(path), 0770); err != nil {
		return "", fmt.Errorf("failed to create dir %s for metatdata of uri: %s to disk, with err: %v", filepath.Dir(path), uri, err)
	}

	f, err := os.Create(path) // creates a file at current directory
	if err != nil {
		return "", fmt.Errorf("failed to create file for metatdata of uri: %s to disk, with err: %v", uri, err)
	}

	defer f.Close()

	if err := ioutil.WriteFile(path, []byte(src.Source), 0644); err != nil {
		return "", fmt.Errorf("failed to write metatdata of uri: %s to disk, with err: %v", uri, err)
	}

	rc.metadataSet[key] = path
	return path, nil

}
