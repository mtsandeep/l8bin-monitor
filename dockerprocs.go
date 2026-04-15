package main

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

var (
	dockerProcCache  DockerProcesses
	dockerProcExpiry time.Time
	dockerProcMu     sync.RWMutex
	dockerProcActive bool
)

var dockerProcessPrefixes = []string{
	"dockerd",
	"docker-proxy",
	"containerd",
	"containerd-shim",
	"ctr",
	"vpnkit",
	"com.docker.backend",
	"com.docker.proxy",
}

func isDockerProcess(name string) bool {
	// Strip .exe on Windows
	if runtime.GOOS == "windows" {
		name = strings.TrimSuffix(name, ".exe")
	}
	for _, prefix := range dockerProcessPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func getDockerProcesses() DockerProcesses {
	dockerProcMu.RLock()
	if time.Now().Before(dockerProcExpiry) {
		cached := dockerProcCache
		dockerProcMu.RUnlock()
		return cached
	}
	dockerProcMu.RUnlock()

	go fetchDockerProcessesInBackground()

	dockerProcMu.RLock()
	cached := dockerProcCache
	dockerProcMu.RUnlock()
	return cached
}

func fetchDockerProcessesInBackground() {
	dockerProcMu.Lock()
	if dockerProcActive {
		dockerProcMu.Unlock()
		return
	}
	dockerProcActive = true
	dockerProcMu.Unlock()

	defer func() {
		dockerProcMu.Lock()
		dockerProcActive = false
		dockerProcMu.Unlock()
	}()

	pids, err := process.Processes()
	if err != nil {
		return
	}

	var processes []DockerProcess
	var totalRAM uint64

	for _, p := range pids {
		name, err := p.Name()
		if err != nil || !isDockerProcess(name) {
			continue
		}

		// Strip .exe for display
		if runtime.GOOS == "windows" {
			name = strings.TrimSuffix(name, ".exe")
		}

		var ram, swap uint64
		if memInfo, err := p.MemoryInfo(); err == nil {
			ram = memInfo.RSS
		}
		if runtime.GOOS == "linux" {
			swap = readProcSwap(p.Pid)
		}

		totalRAM += ram

		processes = append(processes, DockerProcess{
			ID:   containerUID(fmt.Sprintf("%d", p.Pid)),
			Name: name,
			RAM:  ram,
			Swap: swap,
		})
	}

	// Group by name
	groupMap := make(map[string]*DockerProcessGroup)
	for _, p := range processes {
		g, ok := groupMap[p.Name]
		if !ok {
			g = &DockerProcessGroup{Name: p.Name}
			groupMap[p.Name] = g
		}
		g.Count++
		g.RAM += p.RAM
		g.Swap += p.Swap
	}

	groups := make([]DockerProcessGroup, 0, len(groupMap))
	for _, g := range groupMap {
		groups = append(groups, *g)
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].RAM > groups[j].RAM
	})

	sort.Slice(processes, func(i, j int) bool {
		return processes[i].RAM > processes[j].RAM
	})

	dp := DockerProcesses{
		TotalRAM:  totalRAM,
		Groups:    groups,
		Processes: processes,
	}

	dockerProcMu.Lock()
	dockerProcCache = dp
	dockerProcExpiry = time.Now().Add(3 * time.Second)
	dockerProcMu.Unlock()
}

// readProcSwap reads VmSwap directly from /proc/[pid]/status.
// gopsutil's MemoryInfo() only reads /proc/[pid]/statm which lacks swap data.
func readProcSwap(pid int32) uint64 {
	f, err := os.Open(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "VmSwap:") {
			// VmSwap: 0 kB
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return 0
			}
			v, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return 0
			}
			return v * 1024 // kB to bytes
		}
	}
	return 0
}
