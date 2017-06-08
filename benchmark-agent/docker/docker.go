package docker

import (
	"strconv"
	"strings"
	"sync"

	logger "github.com/Sirupsen/logrus"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/hyperpilotio/container-benchmarks/benchmark-agent/model"
)

type Client struct {
	c          *docker.Client
	benchmarks map[string]*DeployedBenchmark
	mutex      sync.RWMutex
}

type DeployedBenchmark struct {
	benchmark *model.Benchmark
	nameToID  map[string]string
}

func NewClient() *Client {
	endpoint := "unix:///var/run/docker.sock"
	c, err := docker.NewClient(endpoint)
	if err != nil {
		panic(err)
	}

	err = c.Ping()
	if err != nil {
		logger.Errorln("Unable to ping docker daemon")
		panic(err)
	}

	return &Client{c: c, benchmarks: make(map[string]*DeployedBenchmark)}
}

func (client *Client) IsCreated(name string) bool {
	client.mutex.RLock()
	defer client.mutex.RUnlock()
	if _, ok := client.benchmarks[name]; ok {
		return true
	}
	return false
}

func (client *Client) DeployedBenchmark(name string) *DeployedBenchmark {
	client.mutex.RLock()
	defer client.mutex.RUnlock()
	if v, ok := client.benchmarks[name]; ok {
		return v
	}
	return nil
}

func (client *Client) DeployBenchmark(benchmark *model.Benchmark) error {
	hostConfig := &docker.HostConfig{
		PublishAllPorts: true,
	}

	deployed := &DeployedBenchmark{
		benchmark: benchmark,
		nameToID:  make(map[string]string),
	}

	logger.Infof("Deploying new benchmark: %v", benchmark)

	parts := strings.Split(benchmark.Image, ":")
	image := parts[0]
	tag := "latest"
	if len(parts) > 1 {
		tag = parts[1]
	}

	// TODO: we may not need to re-pull the image for every new benchmark posted
	logger.Infof("Pulling image %s:%s for benchmark %s", image, tag, benchmark.Name)
	if err := client.c.PullImage(docker.PullImageOptions{
		Repository: image,
		Tag:        tag,
	},
		docker.AuthConfiguration{},
	); err != nil {
		logger.Errorf("Unable to pull image %s:%s for benchmark %s", image, tag, benchmark.Name)
		return err
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
		container, err := client.c.CreateContainer(docker.CreateContainerOptions{
			Name:       containerName,
			Config:     config,
			HostConfig: hostConfig,
		})
		if err != nil {
			logger.Errorf("Unable to create container for benchmark %s", benchmark.Name)
			// Clean up
			client.removeContainers(benchmark.Name)
			return err
		}

		deployed.nameToID[containerName] = container.ID

		err = client.c.StartContainer(container.ID, hostConfig)
		if err != nil {
			logger.Errorf("Unable to start container for benchmark %s", benchmark.Name)
			// Clean up
			client.removeContainers(benchmark.Name)
			return err
		}
	}

	logger.Infof("Successfully deployed containers for benchmark %s", benchmark.Name)
	client.mutex.Lock()
	defer client.mutex.Unlock()
	client.benchmarks[benchmark.Name] = deployed
	return nil
}

func (client *Client) removeContainers(prefix string) {
	// TODO: add code to remove existing containers with names matching the prefix
}

func (client *Client) RemoveDeployedBenchmark(b *DeployedBenchmark) error {
	for _, id := range b.nameToID {
		if err := client.c.RemoveContainer(docker.RemoveContainerOptions{
			ID:            id,
			Force:         true,
			RemoveVolumes: true,
		}); err != nil {
			return err
		}
	}
	client.mutex.Lock()
	defer client.mutex.Unlock()
	delete(client.benchmarks, b.benchmark.Name)
	return nil
}

/*
func (client *Client) UpdateResources(b *DeployedBenchmark, r *model.Resources) error {
	updateOptions := docker.UpdateContainerOptions{}
	if r.CPUShares > 0 {
		updateOptions.CPUShares = int(r.CPUShares)
	}

	if r.Memory > 0 {
		updateOptions.Memory = int(r.Memory)
	}

	logger.Infoln("Updating resources for benchmark", b.benchmark.Name)
	for _, id := range b.nameToID {
		logger.Infoln("Updating container ID %s, %+v", id, updateOptions)
		if err := client.c.UpdateContainer(id, updateOptions); err != nil {
			return err
		}
	}
	return nil
}
*/
