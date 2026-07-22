FROM alpine:3.20

RUN apk add --no-ca-certificates ca-certificates

COPY otcat /usr/local/bin/otcat
COPY otc /usr/local/bin/otc

ENTRYPOINT ["/usr/local/bin/otcat"]
