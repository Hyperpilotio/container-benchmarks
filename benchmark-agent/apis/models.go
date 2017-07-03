package apis

type CgroupConfig struct {
	SetCpuQuota bool `bson:"setCpuQuota" json:"setCpuQuota"`
}

type DurationConfig struct {
	MaxDuration int    `bson:"maxDuration" json:"maxDuration" binding:"required`
	Arg         string `bson:"arg" json:"arg"`
}

type HostConfig struct {
	TargetHost string `bson:"targetHost" json::"targetHost"`
	Arg        string `bson:"arg" json:"arg"`
}

type NetConfig struct {
	MaxBw int    `bson:"maxBw" json::"maxBw"`
	Arg   string `bson:"arg" json:"arg"`
}

type IOConfig struct {
	MaxIO int    `bson:"maxIO" json::"maxIO"`
	Arg   string `bson:"arg" json:"arg"`
}

type Command struct {
	Path string   `bson:"path" json:"path"`
	Args []string `bson:"args" json:"args"`
}

type Benchmark struct {
	Name           string          `bson:"name" json:"name" binding:"required"`
	ResourceType   string          `bson:"resourceType" json:"resourceType" binding:"required"`
	Image          string          `bson:"image" json:"image" binding:"required"`
	Command        Command         `bson:"command" json:"command" binding:"required"`
	Intensity      int             `bson:"intensity" json::"intensity" binding:"required"`
	DurationConfig *DurationConfig `bson:"durationConfig" json:"durationConfig" binding:"required`
	CgroupConfig   *CgroupConfig   `bson:"cgroupConfig" json:"cgroupConfig"`
	HostConfig     *HostConfig     `bson:"hostConfig" json:"hostConfig"`
	NetConfig      *NetConfig      `bson:"netConfig" json:"netConfig"`
	IOConfig       *IOConfig       `bson:"ioConfig" json:"ioConfig"`
	Count          int             `bson:"count" json:"count"`
}

type UpdateRequest struct {
	Intensity int64 `bson:"intensity" json::"intensity" binding:"required"`
}
