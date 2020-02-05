#!/usr/bin/env bash

# Set required go flags
export GO111MODULE=on

# Test if we can build the program
echo "Building kiosk..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build cmd/apiserver/main.go || exit 1
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build cmd/manager/main.go || exit 1

# List packages
PKGS=$(go list ./... | grep -v /vendor/ | grep -v /examples/)

fail=false
for pkg in $PKGS; do
 go test -race -coverprofile=profile.out -covermode=atomic $pkg
 if [ $? -ne 0 ]; then
   fail=true
 fi
done

if [ "$fail" = true ]; then
 echo "Failure"
 exit 1
fi
