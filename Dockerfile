# Stage 0
#
# Build the Go Binary.
#
FROM golang:1.9

ENV CGO_ENABLED 0
RUN mkdir -p /go/src/github.com/wyattjoh/ims
WORKDIR /go/src/github.com/wyattjoh/ims
ADD . /go/src/github.com/wyattjoh/ims

RUN go build -ldflags "-s -w -X main.build=$(git rev-parse HEAD)" -a -tags netgo github.com/wyattjoh/ims/cmd/ims

# Stage 1
#
# Run the Go Binary in Alpine.
#
FROM alpine:3.6
MAINTAINER Wyatt Johnson <wyattjoh@gmail.com>

RUN apk update && \
  apk add \
    ca-certificates \
    mailcap && \
  rm -rf /var/cache/apk/*

COPY --from=0 /go/src/github.com/wyattjoh/ims/ims /bin/
ENTRYPOINT ["/bin/ims"]
