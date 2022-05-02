#!/bin/bash

echo "Starting tests..."

# rm -rf /var/log/output/*

docker run --rm --network quic-net \
  --name goquic-test \
  --link goquic-server \
  --label "com.docker-tc.enabled=0" \
  -v /var/log/output:/var/log/output \
  goquic-client \
  -host goquic-server \
  -env "Local"


docker run --rm --network quic-net \
  --name goquic-test \
  --link goquic-server \
  --label "com.docker-tc.enabled=1" \
  --label "com.docker-tc.delay=10ms" \
  --label "com.docker-tc.loss=1%" \
  --label "com.docker-tc.duplicate=1%" \
  --label "com.docker-tc.corrupt=1%" \
  -v /var/log/output:/var/log/output \
  goquic-client \
  -host goquic-server \
  -env "Local-1"


docker run --rm --network quic-net \
  --name goquic-test \
  --link goquic-server \
  --label "com.docker-tc.enabled=1" \
  --label "com.docker-tc.delay=20ms" \
  --label "com.docker-tc.loss=5%" \
  --label "com.docker-tc.duplicate=5%" \
  --label "com.docker-tc.corrupt=5%" \
  -v /var/log/output:/var/log/output \
  goquic-client \
  -host goquic-server \
  -env "Local-5"



docker run --rm --network quic-net \
  --name goquic-test \
  --link goquic-server \
  --label "com.docker-tc.enabled=1" \
  --label "com.docker-tc.delay=50ms" \
  --label "com.docker-tc.loss=10%" \
  --label "com.docker-tc.duplicate=5%" \
  --label "com.docker-tc.corrupt=10%" \
  -v /var/log/output:/var/log/output \
  goquic-client \
  -host goquic-server \
  -env "Local-10"

