#!/bin/bash

set -x -e

# Install the dependancies.
go install ./...

# Lint.
go get github.com/golang/lint/golint

# Lint the packages.
for package in $(go list ./... | grep -v '/vendor/'); do golint -set_exit_status $package; done

# Vet.
go vet $(go list ./... | grep -v '/vendor/')

# Test.
go test $(go list ./... | grep -v '/vendor/') -v -cover
