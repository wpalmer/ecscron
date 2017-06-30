FROM alpine
ENV SRCPATH=/tmp/gopath/src/github.com/wpalmer/ecscron
ENV RUNDEPS='tzdata ca-certificates'
ENV BUILDDEPS='go git build-base'
ADD *.go $SRCPATH/
RUN apk add --no-cache $RUNDEPS $BUILDDEPS \
 && cd "$SRCPATH" \
 && GOPATH=/tmp/gopath go get -v \
 && GOPATH=/tmp/gopath go build \
 && mv "$SRCPATH/ecscron" /ecscron \
 && cd / \
 && rm -rf /tmp/gopath \
 && apk del $BUILDDEPS
ENTRYPOINT ["/ecscron"]
