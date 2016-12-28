#!/usr/bin/env bash

CGO_ENABLED=0 go build -a -installsuffix cgo
sudo docker build . -t hyperpilot/benchmark-agent
