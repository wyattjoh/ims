FROM alpine:3.24.4
LABEL maintainer="wyattjoh@gmail.com"

RUN apk add --no-cache --virtual ca-certificates mailcap

ARG TARGETPLATFORM
COPY $TARGETPLATFORM/ims /bin/

ENTRYPOINT ["/bin/ims"]
