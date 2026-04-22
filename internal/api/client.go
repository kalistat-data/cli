package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/kalistat-data/cli/internal/keychain"
)

const defaultBaseURL = "https://app.kalistat.com/api/v1"

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

func New() (*Client, error) {
	token, err := keychain.GetToken()
	if err != nil {
		return nil, fmt.Errorf("no API token found — run `kalistat auth login <token>` first: %w", err)
	}
	base := os.Getenv("KALISTAT_API_URL")
	if base == "" {
		base = defaultBaseURL
	}
	return &Client{
		BaseURL: base,
		Token:   token,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// GetJSON performs a GET against path, decodes the body into out (if non-nil),
// and returns the raw body. A non-2xx response becomes an *APIError.
func (c *Client) GetJSON(path string, out any) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, c.BaseURL+path, nil)
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

	body, err := io.ReadAll(resp.Body)
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
