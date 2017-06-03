package main

import (
	"github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
)

func main() {
	endpoint := "unix:///var/run/docker.sock"
	client, err := docker.NewClient(endpoint)
	if err != nil {
		panic(err)
	}

	err = client.Ping()
	if err != nil {
		glog.Error("Unable to ping docker daemon")
		panic(err)
	}

	server := NewServer(client, "7778")
	err = server.Run()
	if err != nil {
		panic(err)
	}
}
