package main

import (
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
	Port         string
	Benchmarks   map[string]*DeployedBenchmark
	mutex        *sync.Mutex
	dockerClient *docker.Client
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

}

func (server *Server) deployBenchmark(benchmark *apis.Benchmark) (*DeployedBenchmark, error) {
	hostConfig := &docker.HostConfig{
		PublishAllPorts: true,
	}

	deployed := &DeployedBenchmark{
		Benchmark: benchmark,
		NameToId:  make(map[string]string),
	}

	glog.Infof("Deploying benchmark: %+v", benchmark)

	parts := strings.Split(benchmark.Image, ":")
	tag := "latest"
	if len(parts) > 1 {
		tag = parts[1]
	}

	err := server.dockerClient.PullImage(docker.PullImageOptions{
		Repository: parts[0],
		Tag:        tag,
	}, docker.AuthConfiguration{})

	if err != nil {
		return nil, err
	}

	for i := 1; i <= benchmark.Count; i++ {
		config := &docker.Config{
			Image: benchmark.Image,
		}

		if benchmark.Command != nil {
			config.Cmd = benchmark.Command
		}

		if benchmark.Resources.CPUShares > 0 {
			config.CPUShares = benchmark.Resources.CPUShares
		}

		if benchmark.Resources.Memory > 0 {
			config.Memory = benchmark.Resources.Memory
		}

		containerName := benchmark.Name + strconv.Itoa(i)
		container, err := server.dockerClient.CreateContainer(docker.CreateContainerOptions{
			Name:       containerName,
			Config:     config,
			HostConfig: hostConfig,
		})

		if err != nil {
			// Clean up
			server.removeContainers(benchmark.Name)
			return nil, err
		}

		deployed.NameToId[containerName] = container.ID

		err = server.dockerClient.StartContainer(container.ID, hostConfig)
		if err != nil {
			// Clean up
			server.removeContainers(benchmark.Name)
			return nil, err
		}
	}

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
			"data":  "Benchmark already created. Please delete benchmark before creating",
		})
		return
	}

	deployed, err := server.deployBenchmark(&benchmark)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": true,
			"data":  "Failed to deploy benchmark: " + err.Error(),
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

func (server *Server) updateResources(c *gin.Context) {
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

	var resources apis.Resources
	if err := c.BindJSON(&resources); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": true,
			"data":  "Unable to deserialize resources: " + err.Error(),
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
		benchmarkGroup.PUT("/:benchmark/resources", server.updateResources)
	}

	return router.Run(":" + server.Port)
}
