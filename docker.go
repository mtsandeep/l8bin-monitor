package main

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"
)

// containerUID derives a stable, opaque 8-char hex ID from the real container
// ID using FNV-1a. Safe to expose — non-reversible, not the actual Docker ID.
func containerUID(realID string) string {
	h := fnv.New32a()
	h.Write([]byte(realID))
	return fmt.Sprintf("%08x", h.Sum32())
}

var dockerClient *http.Client
var dockerEndpoint string

func init() {
	if runtime.GOOS == "windows" {
		// Docker Desktop on Windows exposes a TCP port (Ensure it's enabled in Settings)
		dockerClient = &http.Client{
			Timeout: 4 * time.Second, // Provide more leeway for Docker's slow stream=false computations
		}
		dockerEndpoint = "http://127.0.0.1:2375"
	} else {
		// Target Native Unix VPS environment
		dockerClient = &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return net.Dial("unix", "/var/run/docker.sock")
				},
			},
			Timeout: 4 * time.Second,
		}
		dockerEndpoint = "http://localhost" // For unix dialer, host is overridden to localhost
	}
}

type dockerContainer struct {
	Id     string   `json:"Id"`
	Names  []string `json:"Names"`
	State  string   `json:"State"`
}

type dockerStats struct {
	CPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
		OnlineCPUs     uint64 `json:"online_cpus"`
	} `json:"cpu_stats"`
	PreCPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
	} `json:"precpu_stats"`
	MemoryStats struct {
		Usage uint64            `json:"usage"`
		// Stats map contains cache (cgroupsv1) and inactive_file (cgroupsv2).
		// Docker Desktop / docker stats subtract these to get the working set.
		Stats map[string]uint64 `json:"stats"`
	} `json:"memory_stats"`
}

var (
	containerDiskCache  = make(map[string]int64)
	containerDiskExpiry time.Time
	containerDiskMu     sync.RWMutex
	containerDiskActive bool

	lastContainerStatsCache = make(map[string]singleStat)
	lastContainerStatsMu    sync.Mutex
)

func backgroundFetchSizes() {
	containerDiskMu.Lock()
	if containerDiskActive {
		containerDiskMu.Unlock()
		return
	}
	containerDiskActive = true
	containerDiskMu.Unlock()

	defer func() {
		containerDiskMu.Lock()
		containerDiskActive = false
		containerDiskMu.Unlock()
	}()

	resp, err := dockerClient.Get(fmt.Sprintf("%s/containers/json?size=1&all=1", dockerEndpoint))
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var payload []struct {
			Id         string `json:"Id"`
			SizeRootFs int64  `json:"SizeRootFs"`
		}
		if json.NewDecoder(resp.Body).Decode(&payload) == nil {
			containerDiskMu.Lock()
			for _, c := range payload {
				containerDiskCache[c.Id] = c.SizeRootFs
			}
			containerDiskExpiry = time.Now().Add(1 * time.Hour)
			containerDiskMu.Unlock()
		}
	}
}

func getDockerStats() ([]ContainerStats, error) {
	resp, err := dockerClient.Get(fmt.Sprintf("%s/containers/json?all=1", dockerEndpoint))
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var dcs []dockerContainer
	if err := json.NewDecoder(resp.Body).Decode(&dcs); err != nil {
		return nil, err
	}

	containerDiskMu.RLock()
	expired := time.Now().After(containerDiskExpiry) || len(containerDiskCache) == 0
	containerDiskMu.RUnlock()

	if expired {
		go backgroundFetchSizes()
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make([]ContainerStats, 0, len(dcs))

	for _, dc := range dcs {
		name := dc.Id[:12]
		if len(dc.Names) > 0 {
			name = strings.TrimPrefix(dc.Names[0], "/")
		}

		// Security Prefix Stripping
		if stripPrefix != "" {
			name = strings.TrimPrefix(name, stripPrefix)
		}

		cstat := ContainerStats{
			ID:     containerUID(dc.Id),
			Name:   name,
			Status: dc.State,
		}

		containerDiskMu.RLock()
		cstat.Disk = containerDiskCache[dc.Id]
		containerDiskMu.RUnlock()

		if strings.ToLower(dc.State) == "running" {
			wg.Add(1)
			go func(c dockerContainer, cs ContainerStats) {
				defer wg.Done()
				stat, err := fetchContainerStat(c.Id)
				
				lastContainerStatsMu.Lock()
				if err == nil && stat.mem > 0 {
					cs.CPU = stat.cpu
					cs.Memory = stat.mem
					lastContainerStatsCache[c.Id] = stat
				} else {
					cached := lastContainerStatsCache[c.Id]
					cs.CPU = cached.cpu
					cs.Memory = cached.mem
				}
				lastContainerStatsMu.Unlock()

				mu.Lock()
				results = append(results, cs)
				mu.Unlock()
			}(dc, cstat)
		} else {
			results = append(results, cstat)
		}
	}

	wg.Wait()
	return results, nil
}

type singleStat struct {
	cpu float64
	mem uint64
}

func fetchContainerStat(id string) (singleStat, error) {
	var s singleStat
	resp, err := dockerClient.Get(fmt.Sprintf("%s/containers/%s/stats?stream=false", dockerEndpoint, id))
	if err != nil {
		return s, err
	}
	defer resp.Body.Close()

	var ds dockerStats
	if err := json.NewDecoder(resp.Body).Decode(&ds); err != nil {
		return s, err
	}

	// Match Docker Desktop / docker stats: subtract page cache from raw usage.
	// cgroupsv2 uses inactive_file; cgroupsv1 uses cache.
	rawMem := ds.MemoryStats.Usage
	var pageCache uint64
	if v, ok := ds.MemoryStats.Stats["inactive_file"]; ok && v > 0 {
		pageCache = v // cgroupsv2
	} else if v, ok := ds.MemoryStats.Stats["cache"]; ok {
		pageCache = v // cgroupsv1
	}
	if rawMem > pageCache {
		s.mem = rawMem - pageCache
	} else {
		s.mem = rawMem
	}

	cpuDelta := float64(ds.CPUStats.CPUUsage.TotalUsage - ds.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(ds.CPUStats.SystemCPUUsage - ds.PreCPUStats.SystemCPUUsage)
	
	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpus := float64(ds.CPUStats.OnlineCPUs)
		if cpus == 0 {
			cpus = 1 // Fallback
		}
		s.cpu = (cpuDelta / systemDelta) * cpus * 100.0
	}

	return s, nil
}
