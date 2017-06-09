package main

import (
	"flag"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/fsouza/go-dockerclient"
	"github.com/gin-gonic/gin"
	"github.com/golang/glog"
	"github.com/hyperpilotio/container-benchmarks/benchmark-agent/apis"
)

type Server struct {
	dockerClient *docker.Client
	Port         string
	mutex        *sync.Mutex
	Benchmarks   map[string]*DeployedBenchmark
}

type DeployedBenchmark struct {
	Benchmark *apis.Benchmark
	NameToId  map[string]string
}

func NewServer(client *docker.Client, port string) *Server {
	return &Server{
		dockerClient: client,
		Port:         port,
		mutex:        &sync.Mutex{},
		Benchmarks:   make(map[string]*DeployedBenchmark),
	}
}

func (server *Server) removeContainers(prefix string) {
	// TODO: add code to remove existing containers with names matching the prefix
}

func (server *Server) deployBenchmark(benchmark *apis.Benchmark) (*DeployedBenchmark, error) {
	hostConfig := &docker.HostConfig{
		PublishAllPorts: true,
	}

	deployed := &DeployedBenchmark{
		Benchmark: benchmark,
		NameToId:  make(map[string]string),
	}

	glog.Infof("Deploying new benchmark: %v", benchmark)

	parts := strings.Split(benchmark.Image, ":")
	image := parts[0]
	tag := "latest"
	if len(parts) > 1 {
		tag = parts[1]
	}
	// TODO: we may not need to re-pull the image for every new benchmark posted
	glog.Infof("Pulling image %s:%s for benchmark %s", image, tag, benchmark.Name)
	err := server.dockerClient.PullImage(docker.PullImageOptions{
		Repository: image,
		Tag:        tag,
	}, docker.AuthConfiguration{})

	if err != nil {
		glog.Errorf("Unable to pull image %s:%s for benchmark %s", image, tag, benchmark.Name)
		return nil, err
	}

	config := &docker.Config{
		Image: benchmark.Image,
	}

	config.Cmd = append(config.Cmd, benchmark.Command.Path)
	for _, arg := range benchmark.Command.Args {
		config.Cmd = append(config.Cmd, arg)
	}

	hostConfig.CPUPeriod = 100000 // default CpuPeriod value
	cgroup := &benchmark.CgroupConfig
	if cgroup != nil && cgroup.SetCpuQuota { // use cgroup cpu quota to control benchmark intensity
		hostConfig.CPUQuota = hostConfig.CPUPeriod * benchmark.Intensity / 100
	} else { // pass intensity value directly into benchmark command
		config.Cmd = append(config.Cmd, strconv.Itoa(int(benchmark.Intensity)))
	}

	containerCount := 1
	if benchmark.Count > 0 {
		containerCount = benchmark.Count
	}

	for i := 1; i <= containerCount; i++ {
		containerName := benchmark.Name + strconv.Itoa(i)
		container, err := server.dockerClient.CreateContainer(docker.CreateContainerOptions{
			Name:       containerName,
			Config:     config,
			HostConfig: hostConfig,
		})

		if err != nil {
			glog.Errorf("Unable to create container for benchmark %s", benchmark.Name)
			// Clean up
			server.removeContainers(benchmark.Name)
			return nil, err
		}

		deployed.NameToId[containerName] = container.ID

		err = server.dockerClient.StartContainer(container.ID, hostConfig)
		if err != nil {
			glog.Errorf("Unable to start container for benchmark %s", benchmark.Name)
			// Clean up
			server.removeContainers(benchmark.Name)
			return nil, err
		}
	}

	glog.Infof("Successfully deployed containers for benchmark %s", benchmark.Name)

	return deployed, nil
}

func (server *Server) createBenchmark(c *gin.Context) {
	var benchmark apis.Benchmark
	if err := c.BindJSON(&benchmark); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": true,
			"data":  "Error deserializing benchmark: " + string(err.Error()),
		})
		return
	}

	server.mutex.Lock()
	defer server.mutex.Unlock()
	if _, ok := server.Benchmarks[benchmark.Name]; ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": true,
			"data":  "Benchmark " + benchmark.Name + " already created. Please delete it before re-creating",
		})
		return
	}

	deployed, err := server.deployBenchmark(&benchmark)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": true,
			"data":  "Failed to deploy benchmark " + benchmark.Name + ": " + string(err.Error()),
		})
		return
	}

	server.Benchmarks[benchmark.Name] = deployed
	c.JSON(http.StatusAccepted, gin.H{
		"error": false,
	})
}

func (server *Server) deleteBenchmark(c *gin.Context) {
	benchmarkName := c.Param("benchmark")
	server.mutex.Lock()
	defer server.mutex.Unlock()

	deployed, ok := server.Benchmarks[benchmarkName]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"error": false,
		})
		return
	}

	for i := 1; i <= deployed.Benchmark.Count; i++ {
		err := server.dockerClient.RemoveContainer(docker.RemoveContainerOptions{
			ID:            deployed.NameToId[deployed.Benchmark.Name+strconv.Itoa(i)],
			Force:         true,
			RemoveVolumes: true,
		})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": true,
				"data":  "Unable to remove container: " + err.Error(),
			})
			return
		}
	}

	delete(server.Benchmarks, deployed.Benchmark.Name)

	c.JSON(http.StatusAccepted, gin.H{
		"error": false,
	})
}

func (server *Server) updateIntensity(c *gin.Context) {
	benchmarkName := c.Param("benchmark")
	server.mutex.Lock()
	defer server.mutex.Unlock()
	glog.Infof("Updating resource intensity for benchmark %v", benchmarkName)

	/*
		deployed, ok := server.Benchmarks[benchmarkName]
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{
				"error": false,
			})
			return
		}
			updateOptions := docker.UpdateContainerOptions{}
			if resources.CPUShares > 0 {
				updateOptions.CPUShares = int(resources.CPUShares)
			}

			if resources.Memory > 0 {
				updateOptions.Memory = int(resources.Memory)
			}

			glog.Infof("Updating resources for benchmark", deployed.Benchmark.Name)
			for i := 1; i <= deployed.Benchmark.Count; i++ {
				containerId := deployed.NameToId[deployed.Benchmark.Name+strconv.Itoa(i)]
				glog.Infof("Updating container ID %s, %+v", containerId, updateOptions)
				err := server.dockerClient.UpdateContainer(containerId, updateOptions)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"error": true,
						"data":  "Unable to update resources: " + err.Error(),
					})
					return
				}
			}
	*/

	c.JSON(http.StatusAccepted, gin.H{
		"error": false,
	})
}

func (server *Server) Run() error {
	//gin.SetMode("release")
	router := gin.New()

	// Global middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	benchmarkGroup := router.Group("/benchmarks")
	{
		benchmarkGroup.POST("", server.createBenchmark)
		benchmarkGroup.DELETE("/:benchmark", server.deleteBenchmark)
		benchmarkGroup.PUT("/:benchmark/intensity", server.updateIntensity)
	}

	return router.Run(":" + server.Port)
}

func main() {
	// Calling this to avoid error message "Logging before calling flags.parse"
	flag.CommandLine.Parse([]string{})

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
