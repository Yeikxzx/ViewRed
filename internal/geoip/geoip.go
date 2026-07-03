// Package geoip provides a lightweight GeoIP lookup service using the
// free ip-api.com HTTP endpoint. It caches results in memory so repeat
// lookups for the same IP (common in a topology polling loop) don't
// hit the external API.
package geoip

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// Location holds the geographic coordinates and metadata for an IP.
type Location struct {
	IP        string  `json:"ip"`
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lon"`
	City      string  `json:"city"`
	Region    string  `json:"region"`
	Country   string  `json:"country"`
	CountryCode string `json:"country_code"`
	ISP       string  `json:"isp"`
}

type cacheEntry struct {
	location Location
	fetched  time.Time
}

// Service performs GeoIP lookups with an in-memory cache.
type Service struct {
	cache   map[string]cacheEntry
	mu      sync.RWMutex
	ttl     time.Duration
	client  *http.Client
}

// NewService creates a GeoIP service with the given cache TTL.
func NewService(ttl time.Duration) *Service {
	return &Service{
		cache: make(map[string]cacheEntry),
		ttl:   ttl,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

// ipApiResponse mirrors the JSON returned by ip-api.com/json/{ip}.
type ipApiResponse struct {
	Status     string  `json:"status"`
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
	City       string  `json:"city"`
	RegionName string  `json:"regionName"`
	Country    string  `json:"country"`
	CountryCode string `json:"countryCode"`
	ISP        string  `json:"isp"`
}

// Lookup returns the geographic location for the given IP. Results are
// cached for the service TTL. If the lookup fails the error is returned
// and no cache entry is written.
func (s *Service) Lookup(ip string) (Location, error) {
	if ip == "" {
		return Location{}, fmt.Errorf("empty IP")
	}

	// Check cache
	s.mu.RLock()
	entry, ok := s.cache[ip]
	s.mu.RUnlock()
	if ok && time.Since(entry.fetched) < s.ttl {
		return entry.location, nil
	}

	// Hit ip-api.com (free tier, HTTP, ~45 req/min)
	url := fmt.Sprintf("http://ip-api.com/json/%s?fields=status,lat,lon,city,regionName,country,countryCode,isp", ip)
	resp, err := s.client.Get(url)
	if err != nil {
		return Location{}, fmt.Errorf("geoip request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return Location{}, fmt.Errorf("geoip: ip-api returned %d: %s", resp.StatusCode, string(body))
	}

	var apiResp ipApiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return Location{}, fmt.Errorf("geoip: decode: %w", err)
	}

	if apiResp.Status != "success" {
		return Location{}, fmt.Errorf("geoip: ip-api said %q", apiResp.Status)
	}

	loc := Location{
		IP:         ip,
		Latitude:   apiResp.Lat,
		Longitude:  apiResp.Lon,
		City:       apiResp.City,
		Region:     apiResp.RegionName,
		Country:    apiResp.Country,
		CountryCode: apiResp.CountryCode,
		ISP:        apiResp.ISP,
	}

	// Write cache
	s.mu.Lock()
	s.cache[ip] = cacheEntry{location: loc, fetched: time.Now()}
	s.mu.Unlock()

	return loc, nil
}

// PublicIP discovers this machine's public IP by calling the free
// api.ipify.org endpoint. The same IP is then used for the GeoIP lookup.
func (s *Service) PublicIP() (string, error) {
	resp, err := s.client.Get("https://api.ipify.org?format=text")
	if err != nil {
		return "", fmt.Errorf("public IP discovery: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("public IP read: %w", err)
	}

	ip := string(body)
	if ip == "" {
		return "", fmt.Errorf("public IP: empty response")
	}
	return ip, nil
}

// NetworkLocation returns the geographic location of this network by
// discovering the public IP and looking it up. The result is cached so
// repeated topology polling doesn't cause redundant API calls.
func (s *Service) NetworkLocation() (Location, error) {
	ip, err := s.PublicIP()
	if err != nil {
		return Location{}, fmt.Errorf("network location: %w", err)
	}
	return s.Lookup(ip)
}

// batchRequest represents an item in the ip-api batch POST payload.
type batchRequest struct {
	Query  string `json:"query"`
	Fields string `json:"fields,omitempty"`
}

// LookupBatch resolves multiple IPs with a cache-first strategy.
// Cache misses are batched in groups of 40 with 60s pauses.
// Returns partial results if some lookups fail (error logged, not returned).
func (s *Service) LookupBatch(ips []string) ([]Location, error) {
	var results []Location
	var misses []string

	s.mu.RLock()
	for _, ip := range ips {
		if ip == "" {
			continue
		}
		if entry, ok := s.cache[ip]; ok && time.Since(entry.fetched) < s.ttl {
			results = append(results, entry.location)
		} else {
			misses = append(misses, ip)
		}
	}
	s.mu.RUnlock()

	if len(misses) == 0 {
		return results, nil
	}

	// Deduplicate misses
	uniqueMisses := make(map[string]bool)
	var dedupedMisses []string
	for _, m := range misses {
		if !uniqueMisses[m] {
			uniqueMisses[m] = true
			dedupedMisses = append(dedupedMisses, m)
		}
	}

	batchSize := 40
	for i := 0; i < len(dedupedMisses); i += batchSize {
		end := i + batchSize
		if end > len(dedupedMisses) {
			end = len(dedupedMisses)
		}
		batch := dedupedMisses[i:end]

		if i > 0 {
			// Pause between batches to respect rate limits
			time.Sleep(60 * time.Second)
		}

		var reqPayload []batchRequest
		for _, ip := range batch {
			reqPayload = append(reqPayload, batchRequest{
				Query:  ip,
				Fields: "status,lat,lon,city,regionName,country,countryCode,isp",
			})
		}
		bodyData, _ := json.Marshal(reqPayload)

		resp, err := s.client.Post("http://ip-api.com/batch", "application/json", bytes.NewReader(bodyData))
		if err != nil {
			log.Printf("geoip batch: request failed: %v", err)
			continue
		}
		
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			log.Printf("geoip batch: ip-api returned %d", resp.StatusCode)
			continue
		}

		var batchResps []ipApiResponse
		if err := json.NewDecoder(resp.Body).Decode(&batchResps); err != nil {
			resp.Body.Close()
			log.Printf("geoip batch: decode error: %v", err)
			continue
		}
		resp.Body.Close()

		s.mu.Lock()
		for j, apiResp := range batchResps {
			ip := batch[j] // Order is guaranteed to match by ip-api
			if apiResp.Status != "success" {
				continue
			}
			loc := Location{
				IP:         ip,
				Latitude:   apiResp.Lat,
				Longitude:  apiResp.Lon,
				City:       apiResp.City,
				Region:     apiResp.RegionName,
				Country:    apiResp.Country,
				CountryCode: apiResp.CountryCode,
				ISP:        apiResp.ISP,
			}
			s.cache[ip] = cacheEntry{location: loc, fetched: time.Now()}
			results = append(results, loc)
		}
		s.mu.Unlock()
	}

	return results, nil
}