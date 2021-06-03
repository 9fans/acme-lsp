#!/bin/sh

set -ex

DIR=golang_org_x_tools
REPO=https://go.googlesource.com/tools

# golang.org/x/tools version that gopls/v0.2.2 depends on
COMMIT=952e2c076240

rm -rf $DIR
git clone $REPO
(
	cd tools
	git checkout $COMMIT
)

mkdir $DIR
mv tools/LICENSE $DIR/LICENSE
mv tools/internal/jsonrpc2 $DIR/jsonrpc2
mv tools/internal/span $DIR/span
mv tools/internal/telemetry $DIR/telemetry
mv tools/internal/xcontext $DIR/xcontext
mkdir $DIR/lsp
mv tools/internal/lsp/protocol $DIR/lsp
rm -rf $DIR/lsp/protocol/typescript

(
	cd tools
	echo "Packages in this directory is copied from golang.org/x/tools/internal (commit $COMMIT)."
	#git show -s --format='(commit %h on %ci).'
) > $DIR/README.txt

find $DIR -name '*.go' | xargs sed -i 's!golang.org/x/tools/internal!github.com/fhs/acme-lsp/internal/golang_org_x_tools!'

rm -rf tools
