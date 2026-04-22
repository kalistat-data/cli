package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultBaseURL = "https://app.kalistat.com/api/v1"
	maxBodyBytes   = 10 << 20 // 10 MiB
)

type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// APIError is returned when the API responds with a non-2xx status.
// Body preserves the original JSON so callers that opted into raw
// output can emit it unchanged.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
	Body       []byte
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if txt := http.StatusText(e.StatusCode); txt != "" {
		return fmt.Sprintf("%s (%d)", txt, e.StatusCode)
	}
	return fmt.Sprintf("HTTP %d", e.StatusCode)
}

// NewWithToken constructs a Client with an explicit token and base URL.
// An empty baseURL selects DefaultBaseURL. The base URL is validated:
// scheme must be https, or http only when the host is a loopback address.
func NewWithToken(token, baseURL string) (*Client, error) {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("invalid base URL %q", baseURL)
	}
	switch parsed.Scheme {
	case "https":
	case "http":
		if !isLoopback(parsed.Host) {
			return nil, fmt.Errorf("base URL %q must use https (http allowed only for loopback hosts)", baseURL)
		}
	default:
		return nil, fmt.Errorf("base URL %q must use https or http", baseURL)
	}
	return &Client{
		BaseURL: strings.TrimRight(parsed.String(), "/"),
		Token:   token,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func isLoopback(host string) bool {
	h, _, err := net.SplitHostPort(host)
	if err != nil {
		h = host
	}
	if h == "localhost" {
		return true
	}
	if ip := net.ParseIP(h); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// GetJSON performs a GET against path, decodes the body into out (if non-nil),
// and returns the raw body. A non-2xx response becomes an *APIError.
func (c *Client) GetJSON(path string, out any) ([]byte, error) {
	target, err := url.JoinPath(c.BaseURL, path)
	if err != nil {
		return nil, fmt.Errorf("build URL: %w", err)
	}
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		apiErr := &APIError{StatusCode: resp.StatusCode, Body: body}
		var env struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(body, &env) == nil {
			apiErr.Code = env.Error.Code
			apiErr.Message = env.Error.Message
		}
		return body, apiErr
	}

	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return body, fmt.Errorf("decode response: %w", err)
		}
	}
	return body, nil
}
