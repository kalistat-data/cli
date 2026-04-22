package api

import "time"

type Pagination struct {
	Total    int  `json:"total"`
	Page     int  `json:"page"`
	PageSize int  `json:"page_size"`
	HasMore  bool `json:"has_more"`
}

type Meta struct {
	Count       int         `json:"count,omitempty"`
	Query       string      `json:"query,omitempty"`
	Pagination  *Pagination `json:"pagination,omitempty"`
	GeneratedAt time.Time   `json:"generated_at"`
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

type SearchCategory struct {
	Name   string `json:"name"`
	Key    string `json:"key"`
	Source string `json:"source"`
}

type SearchHit struct {
	Code     string            `json:"code"`
	Name     string            `json:"name"`
	Type     string            `json:"type"`
	Source   string            `json:"source"`
	Category SearchCategory    `json:"category"`
	Links    map[string]string `json:"links"`
}

type SearchResponse struct {
	Data []SearchHit `json:"data"`
	Meta Meta        `json:"meta"`
}

type Observation struct {
	Time  string   `json:"time"`
	Value *float64 `json:"value"`
}

type SeriesDimension struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Position int    `json:"position"`
	Value    string `json:"value"`
}

type Series struct {
	Ticker     string            `json:"ticker"`
	Dimensions []SeriesDimension `json:"dimensions"`
	Values     []Observation     `json:"values"`
}

type SeriesListResponse struct {
	Data []Series `json:"data"`
	Meta Meta     `json:"meta"`
}

type SeriesResponse struct {
	Data Series `json:"data"`
	Meta Meta   `json:"meta"`
}

type Dimension struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Position int    `json:"position"`
}

type Dataset struct {
	Code           string            `json:"code"`
	Name           string            `json:"name"`
	Source         string            `json:"source"`
	DataflowID     string            `json:"dataflow_id"`
	CategoryKey    string            `json:"category_key,omitempty"`
	SeriesCount    int               `json:"series_count,omitempty"`
	Dimensions     []Dimension       `json:"dimensions"`
	TimeDimensions []Dimension       `json:"time_dimensions"`
	Links          map[string]string `json:"links"`
}

type DatasetResponse struct {
	Data Dataset `json:"data"`
	Meta Meta    `json:"meta"`
}

type DimensionValue struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Level int    `json:"level"`
}

type DimensionValuesResponse struct {
	Data []DimensionValue `json:"data"`
	Meta Meta             `json:"meta"`
}

type Category struct {
	Key         string            `json:"key"`
	Name        string            `json:"name"`
	Source      string            `json:"source"`
	HasChildren bool              `json:"has_children"`
	HasDatasets bool              `json:"has_datasets"`
	Children    []Category        `json:"children,omitempty"`
	Datasets    []DatasetStub     `json:"datasets,omitempty"`
	Links       map[string]string `json:"links,omitempty"`
}

type CategoriesListResponse struct {
	Data []Category `json:"data"`
	Meta Meta       `json:"meta"`
}

type CategoryResponse struct {
	Data Category `json:"data"`
	Meta Meta     `json:"meta"`
}

type Ancestor struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Depth int    `json:"depth"`
}

type AncestorsResponse struct {
	Data []Ancestor `json:"data"`
	Meta Meta       `json:"meta"`
}

type DatasetStub struct {
	Code        string            `json:"code"`
	Name        string            `json:"name"`
	Source      string            `json:"source"`
	CategoryKey string            `json:"category_key,omitempty"`
	Links       map[string]string `json:"links,omitempty"`
}

type CategoryDatasetsResponse struct {
	Data []DatasetStub `json:"data"`
	Meta Meta          `json:"meta"`
}
