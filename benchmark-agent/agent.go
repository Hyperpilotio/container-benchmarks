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
	State     string
	Error     string
}

func NewServer(client *docker.Client, port string) *Server {
	return &Server{
		dockerClient: client,
		Port:         port,
		mutex:        &sync.Mutex{},
		Benchmarks:   make(map[string]*DeployedBenchmark),
	}
}

func (server *Server) removeContainers(benchmarkName string) int {
	deployed, ok := server.Benchmarks[benchmarkName]
	if !ok { // no existing containers belong to the given benchmark
		return 0
	}

	deployedContainers := len(deployed.NameToId)
	for containerName, containerId := range deployed.NameToId {
		err := server.dockerClient.RemoveContainer(docker.RemoveContainerOptions{
			ID:            containerId,
			Force:         true,
			RemoveVolumes: true,
		})

		if err != nil {
			glog.Errorf("Unable to remove container %s, %s: ", containerName, containerId, err.Error())
		} else {
			glog.Info("Removed container %s, %s", containerName, containerId)
			deployedContainers--
		}
	}

	if deployedContainers > 0 {
		glog.Errorf("Unable to remove all deployed containers for benchmark %s", benchmarkName)
	} else {
		glog.Infof("Removed all deployed containers for benchmark %s", benchmarkName)
	}

	return deployedContainers
}

func (server *Server) deployBenchmark(deployed *DeployedBenchmark) error {
	benchmark := deployed.Benchmark
	parts := strings.Split(benchmark.Image, ":")
	image := parts[0]
	tag := "latest"
	if len(parts) > 1 {
		tag = parts[1]
	}

	glog.Infof("Pulling image %s:%s for benchmark %s", image, tag, benchmark.Name)
	authConfigs, err := docker.NewAuthConfigurationsFromDockerCfg()
	if err != nil {
		glog.Errorf("Unable to get docker authorization configurations: " + err.Error())
		return err
	}

	authConfig := authConfigs.Configs["auths"]
	err = server.dockerClient.PullImage(docker.PullImageOptions{
		Repository: image,
		Tag:        tag,
	}, authConfig)
	if err != nil {
		glog.Errorf("Unable to pull image %s:%s for benchmark %s: %s", image, tag, benchmark.Name, err.Error())
		return err
	}

	config := &docker.Config{
		Image: benchmark.Image,
	}

	if benchmark.Command.Path != "" {
		config.Cmd = append(config.Cmd, benchmark.Command.Path)
	}
	for _, arg := range benchmark.Command.Args {
		config.Cmd = append(config.Cmd, arg)
	}

	hostConfig := &docker.HostConfig{
		PublishAllPorts: true,
		AutoRemove:      true,
		NetworkMode:     "host",
	}

	cgroupConfig := benchmark.CgroupConfig
	netConfig := benchmark.NetConfig
	durationConfig := benchmark.DurationConfig
	targetHostConfig := benchmark.HostConfig

	if durationConfig != nil {
		// set max time duration for the benchmark run
		maxDuration := strconv.Itoa(durationConfig.MaxDuration)
		glog.Infof("Setting max run duration for benchmark %s to be %s seconds", benchmark.Name, maxDuration)
		if durationConfig.Arg != "" {
			config.Cmd = append(config.Cmd, durationConfig.Arg)
		}
		config.Cmd = append(config.Cmd, maxDuration)
	}

	if cgroupConfig != nil && cgroupConfig.SetCpuQuota {
		// use cgroup cpu quota to control benchmark intensity
		hostConfig.CPUPeriod = 100000
		quota := hostConfig.CPUPeriod * int64(benchmark.Intensity) / 100
		glog.Infof("Setting cpu quota for benchmark %s to be %d", benchmark.Name, quota)
		hostConfig.CPUQuota = quota
	} else if netConfig != nil {
		// set network bandwidth target as benchmark intensity
		netBw := strconv.Itoa(netConfig.MaxBw * benchmark.Intensity / 100)
		netBw += "M"
		glog.Infof("Setting target bandwidth for benchmark %s to be %sbps", benchmark.Name, netBw)
		if netConfig.Arg != "" {
			config.Cmd = append(config.Cmd, netConfig.Arg)
		}
		config.Cmd = append(config.Cmd, netBw)
	} else {
		// pass intensity value directly into benchmark command
		glog.Infof("Setting resource intensity for benchmark %s to be %d", benchmark.Name, benchmark.Intensity)
		config.Cmd = append(config.Cmd, strconv.Itoa(benchmark.Intensity))
	}

	if targetHostConfig != nil {
		// set target host name or ip for the benchmark run
		glog.Infof("Setting target host for benchmark %s to be %s", benchmark.Name, targetHostConfig.TargetHost)
		if targetHostConfig.Arg != "" {
			config.Cmd = append(config.Cmd, targetHostConfig.Arg)
		}
		config.Cmd = append(config.Cmd, targetHostConfig.TargetHost)
	}

	config.Labels = make(map[string]string)
	config.Labels["hyperpilot.io/benchmark-agent"] = "true"

	containerCount := 1
	if benchmark.Count > 0 {
		containerCount = benchmark.Count
	}

	deployed.State = "DEPLOYING"
	for i := 1; i <= containerCount; i++ {
		containerName := benchmark.Name + strconv.Itoa(i)
		container, err := server.dockerClient.CreateContainer(docker.CreateContainerOptions{
			Name:       containerName,
			Config:     config,
			HostConfig: hostConfig,
		})

		if err != nil {
			glog.Errorf("Unable to create container %s for benchmark %s: %s", containerName, benchmark.Name, err.Error())
			// Clean up and remove already-deployed containers
			_ = server.removeContainers(benchmark.Name)
			return err
		}

		deployed.NameToId[containerName] = container.ID

		err = server.dockerClient.StartContainer(container.ID, hostConfig)
		if err != nil {
			glog.Errorf("Unable to start container %s for benchmark %s: %s", containerName, benchmark.Name, err.Error())
			// Clean up and remove already-deployed containers
			_ = server.removeContainers(benchmark.Name)
			return err
		}
	}

	deployed.State = "DEPLOYED"
	glog.Infof("Successfully deployed %d containers for benchmark %s", containerCount, benchmark.Name)

	return nil
}

func (server *Server) createBenchmark(c *gin.Context) {
	var benchmark apis.Benchmark
	if err := c.BindJSON(&benchmark); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": true,
			"data":  "Error deserializing benchmark: " + err.Error(),
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

	deployed := &DeployedBenchmark{
		Benchmark: &benchmark,
		NameToId:  make(map[string]string),
		State:     "CREATING",
	}

	glog.Infof("Creating new benchmark: %+v", benchmark)
	server.Benchmarks[benchmark.Name] = deployed

	go func() {
		err := server.deployBenchmark(deployed)
		if err != nil {
			glog.Errorf("Failed to deploy benchmark: " + err.Error())
			deployed.State = "FAILED"
			deployed.Error = err.Error()
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"error": false,
	})

}

func (server *Server) getBenchmarks(c *gin.Context) {
	server.mutex.Lock()
	defer server.mutex.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"error": false,
		"data":  server.Benchmarks,
	})
}

func (server *Server) getBenchmark(c *gin.Context) {
	benchmarkName := c.Param("benchmark")
	server.mutex.Lock()
	defer server.mutex.Unlock()

	deployed, ok := server.Benchmarks[benchmarkName]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"error": true,
			"data":  "Benchmark " + benchmarkName + " does not exist",
		})
	} else if deployed.State == "FAILED" {
		c.JSON(http.StatusAccepted, gin.H{
			"error": false,
			"data":  "Deployment of benchmark " + benchmarkName + " failed: " + deployed.Error,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"error":  false,
			"status": deployed.State,
		})
	}
}

func (server *Server) deleteBenchmark(c *gin.Context) {
	benchmarkName := c.Param("benchmark")
	server.mutex.Lock()
	defer server.mutex.Unlock()

	deployed, ok := server.Benchmarks[benchmarkName]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"error": true,
			"data":  "Benchmark " + benchmarkName + " does not exist",
		})
		return
	}

	if deployed.State == "FAILED" {
		glog.Warningf("Previous deployment of benchmark %s failed; deleting from server", deployed.Benchmark.Name)
		delete(server.Benchmarks, deployed.Benchmark.Name)
		c.JSON(http.StatusAccepted, gin.H{
			"error": false,
			"data":  "Failed deployment; deleted from server",
		})
		return
	}

	remainingContainers := server.removeContainers(deployed.Benchmark.Name)
	if remainingContainers > 0 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": true,
			"data":  "Unable to remove all containers for benchmark",
		})
	} else {
		c.JSON(http.StatusAccepted, gin.H{
			"error": false,
		})
	}

	delete(server.Benchmarks, deployed.Benchmark.Name)
}

func (server *Server) deleteBenchmarks(c *gin.Context) {
	server.mutex.Lock()
	defer server.mutex.Unlock()

	glog.Infof("Deleting all deployed benchmarks")
	for benchmarkName, deployed := range server.Benchmarks {
		allGone := true

		for i := 1; i <= deployed.Benchmark.Count; i++ {
			containerId := deployed.NameToId[deployed.Benchmark.Name+strconv.Itoa(i)]
			err := server.dockerClient.RemoveContainer(docker.RemoveContainerOptions{
				ID:            containerId,
				Force:         true,
				RemoveVolumes: true,
			})
			if err != nil {
				glog.Errorf("Unable to remove container %s: ", containerId, err.Error())
				allGone = false
			} else {
				glog.Infof("Removed container %s for benchmark %s", containerId, deployed.Benchmark.Name)
			}
		}

		if allGone {
			delete(server.Benchmarks, benchmarkName)
		}
	}

	if len(server.Benchmarks) > 0 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": true,
			"data":  "Unable to remove containers for all benchmarks: ",
		})
	} else {
		c.JSON(http.StatusAccepted, gin.H{
			"error": false,
		})
	}
}

// currently only support updating cpuquota
func (server *Server) updateIntensity(c *gin.Context) {
	benchmarkName := c.Param("benchmark")
	server.mutex.Lock()
	defer server.mutex.Unlock()

	deployed, ok := server.Benchmarks[benchmarkName]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"error": true,
			"data":  "Benchmark does not exist",
		})
		return
	}

	if deployed.State != "DEPLOYED" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": true,
			"data":  "Benchmark has not been deployed",
		})
		return
	}

	var update apis.UpdateRequest
	if err := c.BindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": true,
			"data":  "Unable to deserialize update request: " + err.Error(),
		})
		return
	}

	updateOptions := docker.UpdateContainerOptions{}
	if update.Intensity > 0 {
		updateOptions.CPUPeriod = 100000
		updateOptions.CPUQuota = updateOptions.CPUPeriod * int(update.Intensity) / 100
	}

	glog.Infof("Updating resource intensity for benchmark %s to %d", benchmarkName, update.Intensity)
	for i := 1; i <= deployed.Benchmark.Count; i++ {
		containerId := deployed.NameToId[deployed.Benchmark.Name+strconv.Itoa(i)]
		glog.Infof("Updating container ID %s, %+v", containerId, updateOptions)
		err := server.dockerClient.UpdateContainer(containerId, updateOptions)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": true,
				"data":  "Unable to update resource intensity: " + err.Error(),
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
		benchmarkGroup.GET("/:benchmark", server.getBenchmark)
		benchmarkGroup.GET("", server.getBenchmarks)
		benchmarkGroup.DELETE("/:benchmark", server.deleteBenchmark)
		benchmarkGroup.DELETE("", server.deleteBenchmarks)
		benchmarkGroup.PUT("/:benchmark/intensity", server.updateIntensity)
	}

	return router.Run(":" + server.Port)
}

func main() {
	// Calling this to avoid error message "Logging before calling flags.parse"
	flag.CommandLine.Parse([]string{})

	client, err := docker.NewClientFromEnv()
	if err != nil {
		panic(err)
	}

	err = client.Ping()
	if err != nil {
		glog.Error("Unable to ping docker daemon")
		panic(err)
	}

	filters := make(map[string][]string)
	filters["label"] = []string{"hyperpilot.io/benchmark-agent"}
	existingContainers, err := client.ListContainers(docker.ListContainersOptions{Filters: filters, All: true})
	if err != nil {
		glog.Error("Unable to find existing launched benchmarks")
		panic(err)
	}

	for _, container := range existingContainers {
		err := client.RemoveContainer(docker.RemoveContainerOptions{
			ID:            container.ID,
			Force:         true,
			RemoveVolumes: true,
		})
		if err != nil {
			glog.Errorf("Unable to remove existing container: %v", container)
			panic(err)
		}
		glog.Infof("Removed existing launched benchmark container: %v", container)
	}

	server := NewServer(client, "7778")
	err = server.Run()
	if err != nil {
		panic(err)
	}
}
