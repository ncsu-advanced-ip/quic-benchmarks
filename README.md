# quic-benchmarks

## Local Instructions

### Server
To start the server:
```bash
docker build -t goquic-server -f server/Dockerfile .
docker run --rm --name goquic-server goquic-server
```

### Client

To use the client:
```bash
docker build -t goquic-client -f client/Dockerfile .
docker run --rm --name goquic-client -v /var/log/output:/var/log/output --link goquic-server goquic-client -host goquic-server
```


## Traffic Control

To start the server and client using Traffic Control, we are using `docker-tc`:

Start docker-tc:
```bash
docker run -d \
    --name docker-tc \
    --network host \
    --cap-add NET_ADMIN \
    --restart always \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v /var/docker-tc:/var/docker-tc \
    lukaszlach/docker-tc
```

Start the server on a specific network:
```bash
docker network create quic-net

docker run --rm --network quic-net --name goquic-server goquic-server
```

And then execute the Client:
```bash
docker run --rm --network quic-net \
	--name goquic-test \
	--link goquic-server \
	--label "com.docker-tc.enabled=1" \
	--label "com.docker-tc.limit=1mbps" \
	--label "com.docker-tc.delay=10ms" \
	--label "com.docker-tc.loss=10%" \
	--label "com.docker-tc.duplicate=10%" \
	--label "com.docker-tc.corrupt=10%" \
	-v /var/log/output:/var/log/output \
	goquic-client -host goquic-server
```
