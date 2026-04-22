package api

import "time"

type Meta struct {
	Count       int       `json:"count,omitempty"`
	GeneratedAt time.Time `json:"generated_at"`
}

type Source struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Country *string           `json:"country"`
	RootKey string            `json:"root_key"`
	Links   map[string]string `json:"links"`
}

type SourcesResponse struct {
	Data []Source `json:"data"`
	Meta Meta     `json:"meta"`
}

type RateLimit struct {
	RequestsPerMinute int `json:"requests_per_minute"`
}

type Root struct {
	Name      string            `json:"name"`
	Version   string            `json:"version"`
	Sources   []string          `json:"sources"`
	RateLimit RateLimit         `json:"rate_limit"`
	Links     map[string]string `json:"links"`
}

type RootResponse struct {
	Data Root `json:"data"`
	Meta Meta `json:"meta"`
}
