package main

import (
	"fmt"
	"net/http"
	"time"

	logger "github.com/Sirupsen/logrus"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/hyperpilotio/container-benchmarks/benchmark-agent/model"
)

var corsConfig cors.Config

func init() {
	headers := []string{
		"Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization", "Cache-Control", "X-Requested-With",
		"accept", "origin", "Apitoken",
		"page-size", "page-pos", "order-by", "page-ptr", "total-count", "page-more", "previous-page", "next-page",
	}

	corsConfig = cors.Config{
		AllowMethods:     []string{"POST", "OPTIONS", "GET", "PUT", "DELETE", "UPDATE"},
		AllowHeaders:     headers,
		ExposeHeaders:    headers,
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
	corsConfig.AllowAllOrigins = true
}

type httpServConfig struct {
	Mode string
	Host string
	Port uint16
}

func (c *httpServConfig) String() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func initHTTPServ() {
	initGin(&httpServConfig{
		Mode: "debug",
		Host: "0.0.0.0",
		Port: 7778,
	})
}

func setAPIRoutes(router *gin.Engine) {
	router.POST("benchmarks", createBenchmark)
	router.DELETE("benchmarks/:benchmark", deleteBenchmark)
	router.PUT("benchmarks/:benchmark/intensity", updateIntensity)
}

func createBenchmark(c *gin.Context) {
	var benchmark model.Benchmark
	if err := c.BindJSON(&benchmark); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": true,
			"data":  "Error deserializing benchmark: " + string(err.Error()),
		})
		return
	}

	if dockerClient.IsCreated(benchmark.Name) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": true,
			"data":  "Benchmark " + benchmark.Name + " already created. Please delete it before re-creating",
		})
		return
	}

	if err := dockerClient.DeployBenchmark(&benchmark); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": true,
			"data":  "Failed to deploy benchmark " + benchmark.Name + ": " + string(err.Error()),
		})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"error": false,
	})
}

func deleteBenchmark(c *gin.Context) {
	name := c.Param("benchmark")
	depBenchmark := dockerClient.DeployedBenchmark(name)
	if depBenchmark == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": false,
		})
		return
	}

	if err := dockerClient.RemoveDeployedBenchmark(depBenchmark); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": true,
			"data":  "Unable to remove container: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"error": false,
	})
}

func updateIntensity(c *gin.Context) {
	name := c.Param("benchmark")
	/*
		resources := &model.Resources{}
		if err := c.BindJSON(resources); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": true,
				"data":  "Unable to deserialize resources: " + err.Error(),
			})
			return
		}
	*/

	logger.Infof("Updating resource intensity for benchmark %v", name)
	depBenchmark := dockerClient.DeployedBenchmark(name)
	if depBenchmark == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": false,
		})
		return
	}
	/*
		if err := dockerClient.UpdateResources(depBenchmark, resources); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": true,
				"data":  "Unable to update resources: " + err.Error(),
			})
			return
		}
	*/
	c.JSON(http.StatusAccepted, gin.H{
		"error": false,
	})
}
