FROM golang:1.26-alpine AS build

ARG SERVICE=server
ARG LDFLAGS

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build \
        -ldflags "$LDFLAGS" \
        -o /opt/abf/${SERVICE} ./cmd/${SERVICE}

FROM alpine:3.22

ARG SERVICE=server

LABEL SERVICE="anti-brute-force"

RUN addgroup -S abf && adduser -S abf -G abf

COPY --from=build /opt/abf/${SERVICE} /opt/abf/${SERVICE}
COPY ./configs /etc/abf

USER abf

ENV SERVICE=${SERVICE}
ENV CONFIG_FILE=/etc/abf/config.yaml

ENTRYPOINT ["/bin/sh", "-c", "/opt/abf/${SERVICE} --config=${CONFIG_FILE}"]
