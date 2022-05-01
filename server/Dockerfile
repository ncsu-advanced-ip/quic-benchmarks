# syntax=docker/dockerfile:1

FROM golang:1.18-stretch

WORKDIR /app

RUN mkdir ./server/
COPY server/go.mod ./server/
COPY server/go.sum ./server/
COPY server/*.go ./server/

RUN cd server && go mod download && go build -o /server/goquic-server

RUN mkdir ./client/
COPY client/go.mod ./client/
COPY client/go.sum ./client/
COPY client/*.go ./client/
RUN cd client && go mod download && go build -o /client/goquic-client

RUN mkdir /var/log/output/

# RUN sysctl -w net.core.rmem_max=2500000

ENTRYPOINT [ "/server/goquic-server" ]
