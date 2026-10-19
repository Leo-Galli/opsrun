FROM golang:1.27-alpine AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/opsrun ./cmd/opsrun

FROM alpine:3.21
RUN apk add --no-cache ca-certificates openssh-client && \
    adduser -D -u 10001 opsrun
COPY --from=build /out/opsrun /usr/local/bin/opsrun
USER opsrun
ENTRYPOINT ["opsrun"]
