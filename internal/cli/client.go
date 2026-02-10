package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const defaultURL = "http://localhost:9999"

// Client is an HTTP client for the beads API.
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// NewClientFromEnv creates a Client from BS_URL and BS_TOKEN.
// Values are read from environment variables first, then from a .env
// file in the current directory. Returns an error if BS_TOKEN is not set.
func NewClientFromEnv() (*Client, error) {
	token := getenv("BS_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("BS_TOKEN is required")
	}

	baseURL := getenv("BS_URL")
	if baseURL == "" {
		baseURL = defaultURL
	}

	return &Client{
		BaseURL:    baseURL,
		Token:      token,
		HTTPClient: http.DefaultClient,
	}, nil
}

// Do sends an HTTP request and returns the response body as parsed JSON.
// Returns an error if the response status is not in the 2xx range.
func (c *Client) Do(method, path string, body any) (json.RawMessage, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	url := c.BaseURL + path
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Try to extract error message from JSON response
		var errResp struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error != "" {
			return nil, fmt.Errorf("%s", errResp.Error)
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return json.RawMessage(respBody), nil
}

// prettyJSON formats a json.RawMessage with 2-space indentation.
func prettyJSON(data json.RawMessage) (string, error) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		return "", err
	}
	return buf.String(), nil
}
