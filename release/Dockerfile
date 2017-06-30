FROM alpine
RUN apk add --no-cache tzdata ca-certificates
ADD https://github.com/wpalmer/ecscron/releases/download/v1.2/ecscron /ecscron
RUN chmod a+x /ecscron
ENTRYPOINT ["/ecscron"]
