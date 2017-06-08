package main

import (
	"github.com/hyperpilotio/container-benchmarks/benchmark-agent/docker"
)

var dockerClient *docker.Client

func initDocker() {
	dockerClient = docker.NewClient()
}
