package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type apiError struct {
	StatusCode         int
	Body               string
	Method             string
	Path               string
	RetryAfter         string
	RateLimitLimit     string
	RateLimitRemaining string
	RateLimitReset     string
}

type autoglueClient struct {
	baseURL       string
	orgID         string
	apiKey        string
	orgKey        string
	orgSecret     string
	bearerToken   string
	sendOrgHeader bool
	httpClient    *http.Client
}

type clientConfig struct {
	BaseURL     string
	OrgID       string
	APIKey      string
	OrgKey      string
	OrgSecret   string
	BearerToken string
}

func (e *apiError) hasRateLimitInfo() bool {
	return e.StatusCode == http.StatusTooManyRequests || e.RateLimitRemaining == "0"
}

func (e apiError) Error() string {
	msg := fmt.Sprintf("autoglue API error %d: %s", e.StatusCode, e.Body)
	if e.Method != "" || e.Path != "" {
		msg += fmt.Sprintf(" [%s %s]", e.Method, e.Path)
	}
	if e.hasRateLimitInfo() {
		if e.RateLimitLimit != "" {
			msg += fmt.Sprintf("; X-RateLimit-Limit: %s", e.RateLimitLimit)
		}
		if e.RateLimitRemaining != "" {
			msg += fmt.Sprintf("; X-RateLimit-Remaining: %s", e.RateLimitRemaining)
		}
		if e.RateLimitReset != "" {
			msg += fmt.Sprintf("; X-RateLimit-Reset: %s", e.RateLimitReset)
		}
		if e.RetryAfter != "" {
			msg += fmt.Sprintf("; Retry-After: %s", e.RetryAfter)
		}
	}
	return msg
}

func isNotFound(err error) bool {
	var ae *apiError
	if errors.As(err, &ae) {
		return ae.StatusCode == http.StatusNotFound
	}
	return false
}

func isRateLimited(err error) bool {
	var ae *apiError
	if errors.As(err, &ae) {
		return ae.hasRateLimitInfo()
	}
	return false
}

func newAutoglueClient(cfg clientConfig) (*autoglueClient, error) {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = "https://autoglue.glueopshosted.com/api/v1"
	}
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "https://" + baseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	orgID := strings.TrimSpace(cfg.OrgID)
	apiKey := strings.TrimSpace(cfg.APIKey)
	orgKey := strings.TrimSpace(cfg.OrgKey)
	orgSecret := strings.TrimSpace(cfg.OrgSecret)
	bearerToken := strings.TrimSpace(cfg.BearerToken)

	// Validate org_key/org_secret pairing (xor)
	hasOrgKey := orgKey != ""
	hasOrgSecret := orgSecret != ""
	if hasOrgKey != hasOrgSecret {
		return nil, fmt.Errorf("both org_key and org_secret must be configured together")
	}
	hasOrgCreds := hasOrgKey

	// Must provide one auth method
	hasAPIKey := apiKey != ""
	hasBearer := bearerToken != ""
	if !hasAPIKey && !hasOrgCreds && !hasBearer {
		return nil, fmt.Errorf("one of api_key, (org_key + org_secret), or bearer_token must be configured")
	}

	// org_id required only for api_key or bearer_token
	needsOrgID := hasAPIKey || hasBearer
	if needsOrgID && orgID == "" {
		return nil, fmt.Errorf("org_id must be configured when using api_key or bearer_token")
	}

	return &autoglueClient{
		baseURL:       baseURL,
		orgID:         orgID,
		apiKey:        apiKey,
		orgKey:        orgKey,
		orgSecret:     orgSecret,
		bearerToken:   bearerToken,
		sendOrgHeader: needsOrgID,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

// doJSON performs an HTTP request with a JSON body and decodes a JSON response into out (if non-nil).
func (c *autoglueClient) doJSON(
	ctx context.Context,
	method string,
	path string,
	query string,
	body any,
	out any,
) error {
	url := c.baseURL + path
	if query != "" {
		if !strings.HasPrefix(query, "?") {
			url += "?"
		}
		url += query
	}

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Org scoping: only needed for api_key or bearer_token auth.
	if c.sendOrgHeader {
		req.Header.Set("X-Org-ID", c.orgID)
	}

	// Auth
	if c.apiKey != "" {
		req.Header.Set("X-API-KEY", c.apiKey)
	}
	if c.orgKey != "" {
		req.Header.Set("X-ORG-KEY", c.orgKey)
	}
	if c.orgSecret != "" {
		req.Header.Set("X-ORG-SECRET", c.orgSecret)
	}
	if c.bearerToken != "" {
		// Autoglue uses Bearer tokens in Authorization
		req.Header.Set("Authorization", "Bearer "+c.bearerToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		retryAfter := resp.Header.Get("Retry-After")
		rlLimit := resp.Header.Get("X-RateLimit-Limit")
		rlRemaining := resp.Header.Get("X-RateLimit-Remaining")
		rlReset := resp.Header.Get("X-RateLimit-Reset")

		apiErr := &apiError{
			StatusCode:         resp.StatusCode,
			Body:               string(respBody),
			Method:             method,
			Path:               path,
			RetryAfter:         retryAfter,
			RateLimitLimit:     rlLimit,
			RateLimitRemaining: rlRemaining,
			RateLimitReset:     rlReset,
		}

		if apiErr.hasRateLimitInfo() || retryAfter != "" {
			tflog.Warn(ctx, "Autoglue upstream rate limit indicated", map[string]any{
				"status_code":           resp.StatusCode,
				"method":                method,
				"path":                  path,
				"retry_after":           retryAfter,
				"x_ratelimit_limit":     rlLimit,
				"x_ratelimit_remaining": rlRemaining,
				"x_ratelimit_reset":     rlReset,
			})
		}

		return apiErr
	}

	if out == nil || len(respBody) == 0 {
		return nil
	}

	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}
