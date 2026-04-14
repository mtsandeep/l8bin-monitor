package main

import (
	"sync"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"time"
)

var (
	cpuSmaMutex    sync.Mutex
	cpuHistory     [60]float64 // 60 seconds buffer
	cpuIdx         int
	cpuCount       int
	
	cachedHostInfo *HostInfo
	hostInfoMu     sync.Mutex

	cachedDiskStats DiskStats
	diskStatsExpiry time.Time
	diskStatsMu     sync.RWMutex
	diskStatsActive bool
)

func getSystemStats() (SystemStats, error) {
	var stats SystemStats

	// CPU
	cpuPercents, err := cpu.Percent(0, false)
	if err == nil && len(cpuPercents) > 0 {
		inst := cpuPercents[0]
		stats.CPUBurst = inst
		
		cpuSmaMutex.Lock()
		cpuHistory[cpuIdx] = inst
		cpuIdx = (cpuIdx + 1) % 60
		if cpuCount < 60 {
			cpuCount++
		}

		var sum float64
		for i := 0; i < cpuCount; i++ {
			sum += cpuHistory[i]
		}
		stats.CPUAvg = sum / float64(cpuCount)
		cpuSmaMutex.Unlock()
	}

	// RAM
	v, err := mem.VirtualMemory()
	if err == nil {
		stats.RAM.Total = v.Total
		stats.RAM.Used = v.Used
	}

	// Disk (Cached for 1 hour, fetched in background)
	diskStatsMu.RLock()
	stats.Disk = cachedDiskStats
	expired := time.Now().After(diskStatsExpiry) || cachedDiskStats.Total == 0
	diskStatsMu.RUnlock()

	if expired {
		go fetchDiskStatsInBackground()
	}

	// Load
	l, err := load.Avg()
	if err == nil {
		stats.Load = []float64{l.Load1, l.Load5, l.Load15}
	}

	return stats, nil
}

func getHostInfo() (HostInfo, error) {
	hostInfoMu.Lock()
	defer hostInfoMu.Unlock()

	i, err := host.Info()
	if err != nil {
		return HostInfo{}, err
	}

	if cachedHostInfo == nil {
		h := HostInfo{
			Hostname: i.Hostname,
			OS:       i.OS,
			Platform: i.Platform,
		}
		
		cInfos, err := cpu.Info()
		if err == nil && len(cInfos) > 0 {
			h.CPUModel = cInfos[0].ModelName
		}
		
		cores, err := cpu.Counts(true)
		if err == nil {
			h.CPUCores = cores
		}
		
		cachedHostInfo = &h
	}

	result := *cachedHostInfo
	result.Uptime = i.Uptime
	return result, nil
}

func fetchDiskStatsInBackground() {
	diskStatsMu.Lock()
	if diskStatsActive {
		diskStatsMu.Unlock()
		return
	}
	diskStatsActive = true
	diskStatsMu.Unlock()

	defer func() {
		diskStatsMu.Lock()
		diskStatsActive = false
		diskStatsMu.Unlock()
	}()

	partitions, err := disk.Partitions(false)
	if err != nil {
		return
	}

	var total, used uint64
	for _, p := range partitions {
		usage, err := disk.Usage(p.Mountpoint)
		if err == nil {
			total += usage.Total
			used += usage.Used
		}
	}

	diskStatsMu.Lock()
	cachedDiskStats = DiskStats{
		Total: total,
		Used:  used,
	}
	diskStatsExpiry = time.Now().Add(1 * time.Hour)
	diskStatsMu.Unlock()
}
