package main

import (
	logger "github.com/Sirupsen/logrus"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func initGin(c *httpServConfig) {
	router := NewDefaultJsonEngine(c)
	logger.Infof("Going to start web service. Listen: %s", c)
	mustStartAPIService(router, c)
}

func mustStartAPIService(router *gin.Engine, c *httpServConfig) {
	setAPIRoutes(router)
	if err := router.Run(c.String()); err != nil {
		logger.Panicf("Cannot start web service: %v", err)
	}
}

func NewDefaultJsonEngine(c *httpServConfig) *gin.Engine {
	gin.SetMode(c.Mode)

	router := gin.New()

	router.Use(cors.New(corsConfig))
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	return router
}
