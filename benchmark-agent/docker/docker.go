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

	logger.Infof("Deploying benchmark: %+v", benchmark)

	parts := strings.Split(benchmark.Image, ":")
	tag := "latest"
	if len(parts) > 1 {
		tag = parts[1]
	}

	if err := client.c.PullImage(docker.PullImageOptions{
		Repository: parts[0],
		Tag:        tag,
	},
		docker.AuthConfiguration{},
	); err != nil {
		return err
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
		container, err := client.c.CreateContainer(docker.CreateContainerOptions{
			Name:       containerName,
			Config:     config,
			HostConfig: hostConfig,
		})
		if err != nil {
			// Clean up
			client.removeContainers(benchmark.Name)
			return err
		}

		deployed.nameToID[containerName] = container.ID

		err = client.c.StartContainer(container.ID, hostConfig)
		if err != nil {
			// Clean up
			client.removeContainers(benchmark.Name)
			return err
		}
	}

	client.mutex.Lock()
	defer client.mutex.Unlock()
	client.benchmarks[benchmark.Name] = deployed
	return nil
}

func (client *Client) removeContainers(prefix string) {

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
