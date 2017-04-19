FROM alpine:3.4

RUN apk update && \
  apk add \
    ca-certificates \
    mailcap && \
  rm -rf /var/cache/apk/*

ADD ims /bin/
ENTRYPOINT ["/bin/ims"]
