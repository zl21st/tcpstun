#!/bin/bash

set -e -x

go build -ldflags="-s -w" -o tcpstunc
GOOS=linux GOARCH=arm GOARM=7  go build -ldflags="-s -w" -o tcpstunc_linux_arm
GOOS=linux GOARCH=arm64        go build -ldflags="-s -w" -o tcpstunc_linux_arm64
GOOS=linux GOARCH=amd64        go build -ldflags="-s -w" -o tcpstunc_linux_amd64
