#!/bin/bash
GOOS=windows GOARCH=386 CGO_ENABLED=0 go build -o release/symfs_windows_386.exe -trimpath -ldflags "-s -w -buildid=" ./cmd/main.go
GOOS=windows GOARCH=386 CGO_ENABLED=0 go build -o release/symfs_windows_386_daemon.exe -trimpath -ldflags "-H windowsgui -s -w -buildid=" ./cmd/main.go
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o release/symfs_windows_amd64.exe -trimpath -ldflags "-s -w -buildid=" ./cmd/main.go
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o release/symfs_windows_amd64_daemon.exe -trimpath -ldflags "-H windowsgui -s -w -buildid=" ./cmd/main.go
GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build -o release/symfs_windows_arm64.exe -trimpath -ldflags "-s -w -buildid=" ./cmd/main.go
GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build -o release/symfs_windows_arm64_daemon.exe -trimpath -ldflags "-H windowsgui -s -w -buildid=" ./cmd/main.go