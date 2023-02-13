# ARG BUILDPLATFORM

FROM --platform=$BUILDPLATFORM golang:1.16-alpine as builder

#RUN apt-get update && apt-get install -y gcc-aarch64-linux-gnu

RUN apk add gcc g++
RUN wget -P / https://musl.cc/aarch64-linux-musl-cross.tgz
RUN tar -xvf /aarch64-linux-musl-cross.tgz -C /

COPY . $GOPATH/src/github.com/vilisseranen/temperature-server
WORKDIR $GOPATH/src/github.com/vilisseranen/temperature-server

ARG TARGETOS
ARG TARGETARCH

RUN if [ "${TARGETARCH}" = "arm64" ]; then CC=/aarch64-linux-musl-cross/bin/aarch64-linux-musl-gcc; fi && \
    env GOOS=$TARGETOS GOARCH=$TARGETARCH CGO_ENABLED=1 CC=$CC go build -o /import -v -ldflags="-extldflags=-static"

FROM scratch

COPY --from=builder /import /app

VOLUME ["/data"]

WORKDIR /

ENTRYPOINT ["/app"]
