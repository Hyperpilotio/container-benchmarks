FROM alpine:3.4

ADD benchmark-agent /usr/local/bin/benchmark-agent
COPY .docker /root/.docker

ENTRYPOINT benchmark-agent
