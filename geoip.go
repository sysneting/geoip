package geoip

import (
	"context"
	"fmt"
	"github.com/oschwald/geoip2-golang"
	"net"
	"net/http"
	"sync"
	"time"
)

type Config struct {
	APIKey         string   `json:"apiKey,omitempty"`
	DBPath         string   `json:"dbPath,omitempty"`
	Mode           string   `json:"mode,omitempty"`
	Countries      []string `json:"countries,omitempty"`
	UpdateInterval string   `json:"updateInterval,omitempty"`
	TrustHeaders   bool     `json:"trustHeaders,omitempty"`
}

func CreateConfig() *Config {
	return &Config{
		DBPath:         "/etc/geo/geo.mmdb",
		Mode:           "blacklist",
		UpdateInterval: "24h",
		TrustHeaders:   false,
	}
}

type Plugin struct {
	next         http.Handler
	name         string
	config       *Config
	db           *geoip2.Reader
	dbLock       sync.RWMutex
	updateTicker *time.Ticker
	countries    map[string]struct{}
}

func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	if len(config.Countries) == 0 {
		return nil, fmt.Errorf("countries list cannot be empty")
	}

	if config.Mode != "blacklist" && config.Mode != "whitelist" {
		return nil, fmt.Errorf("invalid mode: %s", config.Mode)
	}

	countries := make(map[string]struct{})
	for _, c := range config.Countries {
		countries[c] = struct{}{}
	}

	p := &Plugin{
		next:      next,
		name:      name,
		config:    config,
		countries: countries,
	}

	if err := p.loadDatabase(); err != nil {
		return nil, err
	}

	duration, err := time.ParseDuration(config.UpdateInterval)
	if err != nil {
		return nil, fmt.Errorf("invalid update interval: %v", err)
	}

	p.updateTicker = time.NewTicker(duration)
	go p.updateDatabase()

	return p, nil
}

func (p *Plugin) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	clientIP := p.getClientIP(req)
	if clientIP == nil {
		p.terminateConnection(rw)
		return
	}

	p.dbLock.RLock()
	record, err := p.db.Country(clientIP)
	p.dbLock.RUnlock()

	if err != nil {
		p.terminateConnection(rw)
		return
	}

	countryCode := record.Country.IsoCode
	_, inList := p.countries[countryCode]

	if (p.config.Mode == "blacklist" && inList) || (p.config.Mode == "whitelist" && !inList) {
		p.terminateConnection(rw)
		return
	}

	p.next.ServeHTTP(rw, req)
}

func (p *Plugin) getClientIP(req *http.Request) net.IP {
	var ip string

	if p.config.TrustHeaders {
		// Priority order: CloudFlare > X-Real-IP > X-Forwarded-For > RemoteAddr
		if cfIP := req.Header.Get("CF-Connecting-IP"); cfIP != "" {
			ip = cfIP
		}
		if ip == "" {
			if realIP := req.Header.Get("X-Real-IP"); realIP != "" {
				ip = realIP
			}
		}
		if ip == "" {
			if forwardedFor := req.Header.Get("X-Forwarded-For"); forwardedFor != "" {
				for i := 0; i < len(forwardedFor) && forwardedFor[i] != ','; i++ {
					ip += string(forwardedFor[i])
				}
			}
		}
	}

	if ip == "" {
		host, _, err := net.SplitHostPort(req.RemoteAddr)
		if err == nil {
			ip = host
		}
	}

	if ip == "" {
		return nil
	}

	return net.ParseIP(ip)
}

func (p *Plugin) terminateConnection(rw http.ResponseWriter) {
	if h, ok := rw.(http.Hijacker); ok {
		if conn, _, err := h.Hijack(); err == nil {
			conn.Close()
			return
		}
	}

	rw.Header().Set("Connection", "close")
	rw.WriteHeader(http.StatusForbidden)
}

func (p *Plugin) loadDatabase() error {
	db, err := geoip2.Open(p.config.DBPath)
	if err != nil {
		return fmt.Errorf("error opening database: %v", err)
	}

	p.dbLock.Lock()
	if p.db != nil {
		p.db.Close()
	}
	p.db = db
	p.dbLock.Unlock()

	return nil
}

func (p *Plugin) updateDatabase() {
	for range p.updateTicker.C {
		if err := p.loadDatabase(); err != nil {
			fmt.Printf("Error updating database: %v\n", err)
		}
	}
}
