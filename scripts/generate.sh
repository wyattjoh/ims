#!/bin/bash

set -x -e

# Generate the Proto Files
go generate $(go list ./... | grep -v '/vendor/')

# Install everything
go install ./... &>/dev/null
RESULT=$?
if [ $RESULT -ne 0 ]; then
  echo failed
  exit 1
fi

# Install godoc2md if we don't already got it.
go get github.com/davecheney/godoc2md

# list all the packges, trim out the vendor directory and any main packages,
# then strip off the package name
PACKAGES="$(go list -f "{{.Name}}:{{.ImportPath}}" ./... | grep -v -E "main:|vendor/" | cut -d ":" -f 2)"

# loop over all packages generating all their documentation
for PACKAGE in $PACKAGES
do

  echo "godoc2md $PACKAGE > $GOPATH/src/$PACKAGE/README.md"

  godoc2md $PACKAGE > $GOPATH/src/$PACKAGE/README.md

done