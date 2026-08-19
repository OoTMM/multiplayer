#!/usr/bin/env bash

cd client
go tool go-winres make --arch amd64
cd ..

rm -rf build
mkdir -p build

LDFLAGS="-s -w -X github.com/OoTMM/multiplayer/shared/version.Version=$VERSION"
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$LDFLAGS" -o build/ootmm-client-win64/OoTMM.exe ./client
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$LDFLAGS" -o build/ootmm-server-win64/OoTMM-Server.exe ./server
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$LDFLAGS" -o build/ootmm-client-linux-amd64/bin/ootmm ./client
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$LDFLAGS" -o build/ootmm-server-linux-amd64/bin/ootmm-server ./server

rm -rf dist
mkdir -p dist
for file in build/*; do
    cd build
    zip -r ../dist/$(basename $file).zip $(basename $file)
    cd ..
done
