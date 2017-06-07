package apis

type Benchmark struct {
	Name              string   `json:"name" binding:"required"`
	ResourceType      string   `json:"resourceType" binding:"required"`
	Image             string   `json:"image" binding:"required"`
	Command           []string `json:"command" binding:"required"`
	ResourceIntensity int      `json:"resourceIntensity" binding:"required"`
	QuotaConfig       bool     `json:"quotaConfig"`
	Count             int      `json:"count"`
}
