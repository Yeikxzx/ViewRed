// Package geoip provides a lightweight GeoIP lookup service using the
// free ip-api.com HTTP endpoint. It caches results in memory so repeat
// lookups for the same IP (common in a topology polling loop) don't
// hit the external API.
package geoip

import (
	"encoding/json"
	"fmt"
	"io"
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