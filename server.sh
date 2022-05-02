#!/bin/bash

echo "Starting TC..."

docker network create quic-net

docker rm -f docker-tc
docker run -d --name docker-tc --network host --cap-add NET_ADMIN --restart always -v /var/run/docker.sock:/var/run/docker.sock -v /var/docker-tc:/var/docker-tc lukaszlach/docker-tc

echo "Starting goquic-server..."
docker rm -f goquic-server
docker run --rm --network quic-net --name goquic-server goquic-server
