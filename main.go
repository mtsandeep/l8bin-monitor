package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

var startTime = time.Now()

var (
	port        int
	interval    time.Duration
	stripPrefix string

	cacheMutex  sync.RWMutex
	cachedStats AllStats
	lastFetch   time.Time

	streamCount atomic.Int32
	subsMutex   sync.Mutex
	subs        = make(map[chan []byte]bool)
	Version     = "dev"
)

func init() {
	flag.IntVar(&port, "port", 5008, "Port to run the HTTP server on")
	flag.DurationVar(&interval, "interval", 1*time.Second, "Interval for caching and streaming stats")
	flag.StringVar(&stripPrefix, "strip-prefix", "", "Prefix string to strip from docker container names")
}

// ensureCache fetches stats if cache is stale, protecting against stampedes
func ensureCache() AllStats {
	cacheMutex.RLock()
	st := cachedStats
	lf := lastFetch
	cacheMutex.RUnlock()

	// If cache is fresh (buffer against race with lazyTicker), return it
	if time.Since(lf) < interval-(100*time.Millisecond) {
		return st
	}

	// Double-checked locking
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	if time.Since(lastFetch) < interval-(100*time.Millisecond) {
		return cachedStats
	}

	stats := fetchAll()
	cachedStats = stats
	lastFetch = time.Now()

	return stats
}

func getMonitorStats() MonitorStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return MonitorStats{
		MemoryMB:   float64(m.Sys) / 1024 / 1024,
		Uptime:     uint64(time.Since(startTime).Seconds()),
		Version:    Version,
		Goroutines: runtime.NumGoroutine(),
	}
}

// fetchAll aggregates system, docker, and monitor stats immediately
func fetchAll() AllStats {
	var all AllStats
	all.Timestamp = time.Now().UnixMilli()

	if sys, err := getSystemStats(); err == nil {
		all.System = sys
	}

	if conts, err := getDockerStats(); err == nil {
		all.Containers = conts
	}

	if hInfo, err := getHostInfo(); err == nil {
		all.Host = hInfo
	}

	all.Monitor = getMonitorStats()

	return all
}

// runLazyTicker runs only when there is at least 1 stream connected
func runLazyTicker() {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if streamCount.Load() == 0 {
			return // Terminate the background loop if no clients
		}
		<-ticker.C
		if streamCount.Load() == 0 {
			return
		}

		all := fetchAll()

		cacheMutex.Lock()
		cachedStats = all
		lastFetch = time.Now()
		cacheMutex.Unlock()

		d, err := json.Marshal(all)
		if err == nil {
			subsMutex.Lock()
			for ch := range subs {
				select {
				case ch <- d: // Non-blocking send
				default:
				}
			}
			subsMutex.Unlock()
		}
	}
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	stats := ensureCache().System
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		log.Printf("Error encoding stats: %v", err)
	}
}

func handleContainers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	conts := ensureCache().Containers
	if err := json.NewEncoder(w).Encode(conts); err != nil {
		log.Printf("Error encoding containers: %v", err)
	}
}

func handleAll(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	all := ensureCache()
	if err := json.NewEncoder(w).Encode(all); err != nil {
		log.Printf("Error encoding all stats: %v", err)
	}
}

func handleStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Register subscriber
	ch := make(chan []byte, 1)
	subsMutex.Lock()
	subs[ch] = true
	subsMutex.Unlock()

	// Handle stream lifecycle and background ticker
	count := streamCount.Add(1)
	if count == 1 {
		go runLazyTicker()
	}

	defer func() {
		streamCount.Add(-1)
		subsMutex.Lock()
		delete(subs, ch)
		subsMutex.Unlock()
		close(ch)
	}()

	// Flush initial state immediately
	initData, _ := json.Marshal(ensureCache())
	fmt.Fprintf(w, "data: %s\n\n", initData)
	flusher.Flush()

	notify := r.Context().Done()

	for {
		select {
		case <-notify:
			return // Client disconnected
		case data := <-ch:
			// Ensure it writes cleanly
			_, err := fmt.Fprintf(w, "data: %s\n\n", data)
			if err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func main() {
	showVersion := flag.Bool("version", false, "Show version and exit")
	flag.BoolVar(showVersion, "v", false, "Show version and exit (shorthand)")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Litebin Stats Monitor %s\n", Version)
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/stats", handleStats)
	mux.HandleFunc("/containers", handleContainers)
	mux.HandleFunc("/all", handleAll)
	mux.HandleFunc("/stream", handleStream)

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	log.Printf("Starting Litebin Stats Monitor on http://%s with interval %v", addr, interval)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
