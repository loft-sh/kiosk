#!/bin/bash

go run cmd/apiserver/main.go --secure-port=9443 --insecure-port=8080 --insecure-bind-address=127.0.0.1 --delegated-auth=false
