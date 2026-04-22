package api

import (
	"encoding/json"
	"errors"
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
	safe := redactURL(baseURL, parsed)
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("invalid base URL %q", safe)
	}
	switch parsed.Scheme {
	case "https":
	case "http":
		if !isLoopback(parsed.Host) {
			return nil, fmt.Errorf("base URL %q must use https (http allowed only for loopback hosts)", safe)
		}
	default:
		return nil, fmt.Errorf("base URL %q must use https or http", safe)
	}
	// Strip any userinfo from the stored base URL so we never accidentally
	// send embedded credentials as Basic auth alongside the Bearer token.
	parsed.User = nil
	return &Client{
		BaseURL: strings.TrimRight(parsed.String(), "/"),
		Token:   token,
		HTTP: &http.Client{
			Timeout:       30 * time.Second,
			CheckRedirect: safeRedirect,
		},
	}, nil
}

// safeRedirect enforces two invariants before following a redirect:
//   - never downgrade from https to http (would send the Bearer token in
//     cleartext if the same host also serves plain HTTP);
//   - never follow to a different host (the token belongs to the original
//     host only — Go's default policy strips Authorization cross-host, but
//     we refuse the redirect entirely so the user notices).
//
// Also caps redirect chains at 10, matching Go's default.
func safeRedirect(req *http.Request, via []*http.Request) error {
	if len(via) == 0 {
		return nil
	}
	original := via[0]
	if original.URL.Scheme == "https" && req.URL.Scheme != "https" {
		return fmt.Errorf("refusing redirect from https to %s (would expose credentials)", req.URL.Scheme)
	}
	if req.URL.Host != original.URL.Host {
		return fmt.Errorf("refusing cross-host redirect to %s", req.URL.Host)
	}
	if len(via) >= 10 {
		return errors.New("too many redirects")
	}
	return nil
}

// redactURL returns a form of `raw` safe for echoing in error messages:
// any userinfo (e.g. `https://user:secret@host`) is replaced with a
// REDACTED placeholder so embedded passwords don't leak into stderr.
func redactURL(raw string, parsed *url.URL) string {
	if parsed == nil || parsed.User == nil {
		return raw
	}
	copy := *parsed
	copy.User = url.UserPassword(parsed.User.Username(), "REDACTED")
	return copy.String()
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

// GetJSON performs a GET against path (joined to the base URL) with the given
// query, decodes the body into out (if non-nil), and returns the raw body.
// A non-2xx response becomes an *APIError. `query` may be nil.
func (c *Client) GetJSON(path string, query url.Values, out any) ([]byte, error) {
	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("base URL: %w", err)
	}
	u := base.JoinPath(path)
	if query != nil {
		u.RawQuery = query.Encode()
	}
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
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
