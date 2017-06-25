FROM alpine
RUN apk add --no-cache tzdata ca-certificates
ADD ecscron /ecscron
ENTRYPOINT ["/ecscron"]
