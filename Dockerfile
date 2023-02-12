FROM --platform=$BUILDPLATFORM golang:1.16 as builder

RUN apt-get update && apt-get install -y gcc-aarch64-linux-gnu

COPY . $GOPATH/src/github.com/vilisseranen/temperature-server
WORKDIR $GOPATH/src/github.com/vilisseranen/temperature-server

ARG TARGETOS
ARG TARGETARCH

RUN if [ "${TARGETARCH}" = "arm64" ]; then CC=aarch64-linux-gnu-gcc; fi && \
    env GOOS=$TARGETOS GOARCH=$TARGETARCH CGO_ENABLED=1 CC=$CC go build -ldflags='-s -w -extldflags "-static"' -o /go/bin/import

FROM scratch

COPY --from=builder /go/bin/import /app

VOLUME ["/data"]

WORKDIR /

ENTRYPOINT ["/app"]
