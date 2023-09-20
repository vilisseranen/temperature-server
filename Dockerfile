FROM --platform=$BUILDPLATFORM golang:1.20-alpine as builder

RUN apk add gcc g++

COPY . $GOPATH/src/github.com/vilisseranen/temperature-server
WORKDIR $GOPATH/src/github.com/vilisseranen/temperature-server

ARG TARGETOS
ARG TARGETARCH

RUN GOOS=$TARGETOS GOARCH=$TARGETARCH CGO_ENABLED=0 go build -o /import -v -ldflags="-w -s"

FROM scratch

COPY --from=builder /import /app

VOLUME ["/data"]

ENTRYPOINT ["/app"]
