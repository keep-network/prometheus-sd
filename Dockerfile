FROM golang:1.18-alpine AS base

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY ./ ./

RUN go build -o keep-sd

FROM alpine as runtime

ENV BIN_PATH=/usr/local/bin

COPY --from=base /app/keep-sd $BIN_PATH

VOLUME [ "/data" ]

ENTRYPOINT [ "keep-sd" ]
