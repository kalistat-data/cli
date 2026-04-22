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

func (c *Client) GetJSON(path string, out any) error {
	req, err := http.NewRequest(http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("GET %s: %s: %s", path, resp.Status, string(body))
	}
	return json.Unmarshal(body, out)
}
