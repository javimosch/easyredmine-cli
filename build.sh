#!/bin/bash
set -e
go build -ldflags="-s -w" -o easyredmine-cli main.go
ls -lh easyredmine-cli
