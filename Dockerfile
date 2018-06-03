FROM alpine:3.6
MAINTAINER Wyatt Johnson <wyattjoh@gmail.com>

RUN apk add --no-cache --virtual ca-certificates mailcap

COPY ims /bin/

ENTRYPOINT ["/bin/ims"]
