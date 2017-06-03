package main

import (
	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
)

var dockerClient *docker.Client

func initDocker() {
	endpoint := "unix:///var/run/docker.sock"
	c, err := docker.NewClient(endpoint)
	if err != nil {
		panic(err)
	}

	err = c.Ping()
	if err != nil {
		glog.Error("Unable to ping docker daemon")
		panic(err)
	}

	dockerClient = c
}
