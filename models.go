package main

type RAMStats struct {
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

type AllStats struct {
	System     SystemStats      `json:"system"`
	Containers []ContainerStats `json:"containers"`
	Host       HostInfo         `json:"host"`
	Monitor    MonitorStats     `json:"monitor"`
	Timestamp  int64            `json:"timestamp"`
}
