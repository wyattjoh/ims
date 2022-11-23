FROM alpine:3.17.0
LABEL maintainer="wyattjoh@gmail.com"

RUN apk add --no-cache --virtual ca-certificates mailcap

COPY ims /bin/

ENTRYPOINT ["/bin/ims"]
