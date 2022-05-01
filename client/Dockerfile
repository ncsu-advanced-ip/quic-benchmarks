# syntax=docker/dockerfile:1

FROM golang:1.18-stretch

WORKDIR /app

COPY client/go.mod ./
COPY client/go.sum ./
COPY client/*.go ./

RUN go mod download
RUN go build -o /goquic-client

# RUN sysctl -w net.core.rmem_max=2500000

ENTRYPOINT [ "/goquic-client" ]
