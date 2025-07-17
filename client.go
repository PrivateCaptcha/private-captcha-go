package privatecaptcha

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jpillora/backoff"
)

var (
	headerApiKey     = http.CanonicalHeaderKey("X-Api-Key")
	headerTraceID    = http.CanonicalHeaderKey("X-Trace-ID")
	retryAfterHeader = http.CanonicalHeaderKey("Retry-After")
	errEmptyAPIKey   = errors.New("privatecaptcha: API key is empty")
	errAPIError      = errors.New("privatecaptcha: unexpected API error")
	ErrOverloaded    = errors.New("privatecaptcha: server is overloaded")
)

const (
	GlobalDomain     = "api.privatecaptcha.com"
	EUDomain         = "api.eu.privatecaptcha.com"
	DefaultFormField = "private-captcha-solution"
	maxErrLength     = 140
)

type Configuration struct {
	Domain    string
	APIKey    string
	FormField string
	Client    *http.Client
}

type Client struct {
	endpoint  string
	apiKey    string
	formField string
	client    *http.Client
}

// NewClient creates a new instance of Private Captcha API client
func NewClient(cfg Configuration) (*Client, error) {
	if len(cfg.APIKey) == 0 {
		return nil, errEmptyAPIKey
	}

	if len(cfg.Domain) == 0 {
		cfg.Domain = GlobalDomain
	} else if strings.HasPrefix(cfg.Domain, "http") {
		cfg.Domain = strings.TrimPrefix(cfg.Domain, "https://")
		cfg.Domain = strings.TrimPrefix(cfg.Domain, "http://")
	}

	if cfg.Client == nil {
		cfg.Client = http.DefaultClient
	}

	if len(cfg.FormField) == 0 {
		cfg.FormField = DefaultFormField
	}

	return &Client{
		endpoint:  fmt.Sprintf("https://%s/verify", strings.Trim(cfg.Domain, "/")),
		apiKey:    cfg.APIKey,
		client:    cfg.Client,
		formField: cfg.FormField,
	}, nil
}

// retriableError is a wrapper for errors that should be retried.
type retriableError struct {
	err error
}

func (e retriableError) Error() string {
	return e.err.Error()
}

func (e retriableError) Unwrap() error {
	return e.err
}

func (c *Client) doVerify(ctx context.Context, solution string) (*VerifyOutput, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, strings.NewReader(solution))
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set(headerApiKey, c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, 0, retriableError{err}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		seconds := -1
		if retryAfter := resp.Header.Get(retryAfterHeader); len(retryAfter) > 0 {
			if value, err := strconv.Atoi(retryAfter); err == nil {
				seconds = value
			}
		}

		return nil, seconds, retriableError{ErrOverloaded}
	}

	if (resp.StatusCode >= 500) ||
		(resp.StatusCode == http.StatusTooManyRequests) ||
		(resp.StatusCode == http.StatusRequestTimeout) ||
		(resp.StatusCode == http.StatusTooEarly) {
		return nil, 0, retriableError{errAPIError}
	}

	if resp.StatusCode >= 300 {
		return nil, 0, fmt.Errorf("privatecaptcha: API request failed with code %v", resp.StatusCode)
	}

	response := &VerifyOutput{requestID: resp.Header.Get(headerTraceID)}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return response, 0, retriableError{err}
	}

	return response, 0, nil
}

type VerifyInput struct {
	Solution          string
	MaxBackoffSeconds int
	Attempts          int
}

// Verify will verify CAPTCHA solution obtained from the client-side. Solution usually comes as part of the form.
// In case of errors, can use VerificationResponse.RequestID() for tracing. Do NOT retry on ErrOverloaded.
func (c *Client) Verify(ctx context.Context, input VerifyInput) (*VerifyOutput, error) {
	attempts := 5
	if input.Attempts > 0 {
		attempts = input.Attempts
	}

	maxBackoffSeconds := 4
	if input.MaxBackoffSeconds > 0 {
		maxBackoffSeconds = input.MaxBackoffSeconds
	}

	b := &backoff.Backoff{
		Min:    200 * time.Millisecond,
		Max:    time.Duration(maxBackoffSeconds) * time.Second,
		Factor: 2,
		Jitter: true,
	}

	var response *VerifyOutput
	var err error
	var seconds int

	for i := 0; i < attempts; i++ {
		response, seconds, err = c.doVerify(ctx, input.Solution)

		var rerr retriableError
		if (err != nil) && errors.As(err, &rerr) && (attempts > 1) {
			backoffDuration := b.Duration()
			if int64(seconds)*1000 > backoffDuration.Milliseconds() {
				backoffDuration = time.Duration(min(seconds, maxBackoffSeconds)) * time.Second
			}
			time.Sleep(backoffDuration)
		} else {
			break
		}
	}

	return response, err
}

// VerifyRequest fetches puzzle solution from HTTP form field configured on creation and calls Verify() with defaults
func (c *Client) VerifyRequest(ctx context.Context, r *http.Request) (*VerifyOutput, error) {
	solution := r.FormValue(c.formField)
	return c.Verify(ctx, VerifyInput{Solution: solution})
}
