package main

type RAMStats struct {
	Used  uint64 `json:"used"`
	Total uint64 `json:"total"`
}

type SwapStats struct {
	Used  uint64 `json:"used"`
	Total uint64 `json:"total"`
}

type DiskStats struct {
	Used  uint64 `json:"used"`
	Total uint64 `json:"total"`
}

type SystemStats struct {
	CPUBurst float64   `json:"cpu_burst"`
	CPUAvg   float64   `json:"cpu_avg"`
	RAM      RAMStats  `json:"ram"`
	Swap     SwapStats `json:"swap"`
	Disk     DiskStats `json:"disk"`
	Load     []float64 `json:"load"`
}

type ContainerStats struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Status string  `json:"status"`
	CPU    float64 `json:"cpu"`
	Memory uint64  `json:"memory"`
	Disk   int64   `json:"disk"`
}

type HostInfo struct {
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Platform string `json:"platform"`
	Uptime   uint64 `json:"uptime"`
	CPUModel string `json:"cpuModel"`
	CPUCores int    `json:"cpuCores"`
}

type MonitorStats struct {
	MemoryMB  float64 `json:"memory_mb"`
	Uptime    uint64  `json:"uptime"`
	Version   string  `json:"version"`
	Goroutines int    `json:"goroutines"`
}

type DockerProcess struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	RAM  uint64 `json:"ram"`
	Swap uint64 `json:"swap"`
}

type DockerProcessGroup struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
	RAM   uint64 `json:"ram"`
	Swap  uint64 `json:"swap"`
}

type DockerProcesses struct {
	TotalRAM  uint64               `json:"total_ram"`
	Groups    []DockerProcessGroup `json:"groups"`
	Processes []DockerProcess      `json:"processes"`
}

type AllStats struct {
	System      SystemStats      `json:"system"`
	Containers  []ContainerStats `json:"containers"`
	DockerProcs DockerProcesses  `json:"docker_procs"`
	Host        HostInfo         `json:"host"`
	Monitor     MonitorStats     `json:"monitor"`
	Timestamp   int64            `json:"timestamp"`
}
