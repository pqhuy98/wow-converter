package casc

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/pqhuy98/wow-converter/internal/wow/constants"
	"github.com/pqhuy98/wow-converter/internal/wow/formats"
	"github.com/pqhuy98/wow-converter/internal/wow/log"
)

type rankedHost struct {
	host string
	ping int
}

type resolutionCacheEntry struct {
	promise     chan struct{}
	bestHost    *rankedHost
	rankedHosts []rankedHost
	done        bool
	err         error
}

// CDNResolver resolves CDN hosts with ping-based ranking.
type CDNResolver struct {
	resolutionCache map[string]*resolutionCacheEntry
	failedHosts     map[string]struct{}
	mu              sync.Mutex
}

// DefaultCDNResolver is the shared CDN resolver instance.
var DefaultCDNResolver = &CDNResolver{
	resolutionCache: map[string]*resolutionCacheEntry{},
	failedHosts:     map[string]struct{}{},
}

// StartPreResolution starts CDN pre-resolution for a region.
func (r *CDNResolver) StartPreResolution(region string, product string) {
	if product == "" {
		product = "wow"
	}
	log.Write("Starting CDN pre-resolution for region: %s", region)
	go func() {
		host := fmt.Sprintf(constants.Patch.Host, region)
		url := host + product + constants.Patch.ServerConfig
		res, err := formats.Get(url)
		if err != nil || !res.OK {
			if err != nil {
				log.Write("Failed to pre-resolve CDN hosts for region %s: %s", region, err.Error())
			}
			return
		}
		serverConfigs := ParseVersionConfig(string(res.Body))
		var serverConfig VersionConfigEntry
		for _, e := range serverConfigs {
			if e["Name"] == region {
				serverConfig = e
				break
			}
		}
		if serverConfig == nil {
			log.Write("Failed to pre-resolve CDN hosts for region %s: CDN config does not contain entry for region %s", region, region)
			return
		}
		_, _ = r.GetBestHost(region, serverConfig)
	}()
}

// GetBestHost returns the best CDN host for a region.
func (r *CDNResolver) GetBestHost(region string, serverConfig VersionConfigEntry) (string, error) {
	cacheKey := r.cacheKey(region, serverConfig["Hosts"])
	r.mu.Lock()
	cached := r.resolutionCache[cacheKey]
	r.mu.Unlock()

	if cached != nil && cached.done && cached.bestHost != nil {
		log.Write("Using cached CDN host for %s: %s", region, cached.bestHost.host)
		return cached.bestHost.host + serverConfig["Path"] + "/", nil
	}
	if cached != nil && cached.promise != nil {
		log.Write("Waiting for CDN resolution for %s", region)
		<-cached.promise
		r.mu.Lock()
		entry := r.resolutionCache[cacheKey]
		r.mu.Unlock()
		if entry.err != nil {
			return "", entry.err
		}
		return entry.rankedHosts[0].host + serverConfig["Path"] + "/", nil
	}

	log.Write("Resolving CDN hosts for %s: %s", region, serverConfig["Hosts"])
	entry := &resolutionCacheEntry{promise: make(chan struct{})}
	r.mu.Lock()
	r.resolutionCache[cacheKey] = entry
	r.mu.Unlock()

	rankedHosts, err := r.resolveHosts(region, serverConfig)
	if err != nil {
		entry.err = err
		close(entry.promise)
		entry.done = true
		return "", err
	}
	entry.rankedHosts = rankedHosts
	entry.bestHost = &rankedHosts[0]
	entry.done = true
	close(entry.promise)
	return rankedHosts[0].host + serverConfig["Path"] + "/", nil
}

// GetRankedHosts returns all available hosts ranked by ping speed.
func (r *CDNResolver) GetRankedHosts(region string, serverConfig VersionConfigEntry) ([]string, error) {
	cacheKey := r.cacheKey(region, serverConfig["Hosts"])
	r.mu.Lock()
	cached := r.resolutionCache[cacheKey]
	r.mu.Unlock()

	if cached != nil && cached.done && cached.rankedHosts != nil {
		log.Write("Using cached ranked CDN hosts for %s", region)
		return mapHosts(cached.rankedHosts, serverConfig["Path"]), nil
	}
	if cached != nil && cached.promise != nil {
		log.Write("Waiting for CDN resolution for %s", region)
		<-cached.promise
		r.mu.Lock()
		entry := r.resolutionCache[cacheKey]
		r.mu.Unlock()
		if entry.err != nil {
			return nil, entry.err
		}
		return mapHosts(entry.rankedHosts, serverConfig["Path"]), nil
	}

	log.Write("Resolving CDN hosts for %s: %s", region, serverConfig["Hosts"])
	entry := &resolutionCacheEntry{promise: make(chan struct{})}
	r.mu.Lock()
	r.resolutionCache[cacheKey] = entry
	r.mu.Unlock()

	rankedHosts, err := r.resolveHosts(region, serverConfig)
	if err != nil {
		entry.err = err
		close(entry.promise)
		entry.done = true
		return nil, err
	}
	entry.rankedHosts = rankedHosts
	entry.bestHost = &rankedHosts[0]
	entry.done = true
	close(entry.promise)
	return mapHosts(rankedHosts, serverConfig["Path"]), nil
}

// MarkHostFailed marks a host as failed.
func (r *CDNResolver) MarkHostFailed(host string) {
	log.Write("Marking CDN host as failed: %s", host)
	r.mu.Lock()
	r.failedHosts[host] = struct{}{}
	r.mu.Unlock()
}

// ClearCache clears resolver caches.
func (r *CDNResolver) ClearCache() {
	r.mu.Lock()
	r.resolutionCache = map[string]*resolutionCacheEntry{}
	r.failedHosts = map[string]struct{}{}
	r.mu.Unlock()
}

func (r *CDNResolver) cacheKey(region, hosts string) string {
	return region + "|" + hosts
}

func (r *CDNResolver) resolveHosts(region string, serverConfig VersionConfigEntry) ([]rankedHost, error) {
	log.Write("Resolving best host for %s: %s", region, serverConfig["Hosts"])
	hosts := strings.Fields(serverConfig["Hosts"])
	var validHosts []rankedHost
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, h := range hosts {
		host := "https://" + h + "/"
		r.mu.Lock()
		_, failed := r.failedHosts[host]
		r.mu.Unlock()
		if failed {
			log.Write("Skipping previously failed host: %s", host)
			continue
		}
		wg.Add(1)
		go func(h string) {
			defer wg.Done()
			pingMs, err := formats.Ping(h)
			if err != nil {
				log.Write("Host %s failed to resolve a ping: %s", h, err.Error())
				return
			}
			log.Write("Host %s resolved with %dms ping", h, pingMs)
			mu.Lock()
			validHosts = append(validHosts, rankedHost{host: h, ping: pingMs})
			mu.Unlock()
		}(host)
	}
	wg.Wait()
	if len(validHosts) == 0 {
		return nil, fmt.Errorf("unable to resolve any CDN hosts (all failed or blocked)")
	}
	sort.Slice(validHosts, func(i, j int) bool { return validHosts[i].ping < validHosts[j].ping })
	log.Write("%s resolved as the fastest host with a ping of %dms", validHosts[0].host, validHosts[0].ping)
	return validHosts, nil
}

func mapHosts(hosts []rankedHost, path string) []string {
	out := make([]string, len(hosts))
	for i, h := range hosts {
		out[i] = h.host + path + "/"
	}
	return out
}
