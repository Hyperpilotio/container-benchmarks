package apis

type Resources struct {
	CPUShares int64 `json:"cpushares"`
	Memory    int64 `json:"memory"`
}

type Benchmark struct {
	Name      string    `json:"name" binding:"required"`
	Count     int       `json:"count" binding:"required"`
	Resources Resources `json:"resources"`
	Image     string    `json:"image" binding:"required"`
	Command   []string  `json:"command"`
}
