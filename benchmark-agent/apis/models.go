package apis

type CgroupConfig struct {
	SetCpuQuota bool `json:"setCpuQuota"`
}

type NetConfig struct {
	SetBwLimit bool `json:"setBwLimit"`
	MaxBw      int  `json::"maxBw"`
}

type Command struct {
	Path string   `json:"path" binding:"required"`
	Args []string `json:"args"`
}

type Benchmark struct {
	Name         string       `json:"name" binding:"required"`
	ResourceType string       `json:"resourceType" binding:"required"`
	Image        string       `json:"image" binding:"required"`
	Command      Command      `json:"command" binding:"required"`
	Intensity    int          `json::"intensity" binding:"required"`
	CgroupConfig CgroupConfig `json:"cgroupConfig"`
	NetConfig    NetConfig    `json:"netConfig"`
	Count        int          `json:"count"`
}

type UpdateRequest struct {
	Intensity int64 `json::"intensity" binding:"required"`
}
