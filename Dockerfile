FROM alpine:3.15.1
LABEL maintainer="wyattjoh@gmail.com"

RUN apk add --no-cache --virtual ca-certificates mailcap

COPY ims /bin/

ENTRYPOINT ["/bin/ims"]
