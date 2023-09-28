#!/bin/bash

set -e -x

go build -ldflags="-s -w" -o tcpstuns
#GOOS=linux GOARCH=arm GOARM=7  go build -ldflags="-s -w" -o tcpstuns_linux_arm
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o tcpstuns_linux_amd64
