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
)

const (
	GlobalDomain     = "api.privatecaptcha.com"
	EUDomain         = "api.eu.privatecaptcha.com"
	DefaultFormField = "private-captcha-solution"
	maxErrLength     = 140
)

// HTTPError represents an error with an associated HTTP status code
type HTTPError struct {
	StatusCode int
}

func (e HTTPError) Error() string {
	return fmt.Sprintf("privatecaptcha: HTTP error %d", e.StatusCode)
}

// GetStatusCode returns the HTTP status code if the error is an HTTPError
func GetStatusCode(err error) (int, bool) {
	var httpErr HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode, true
	}
	return 0, false
}

type Configuration struct {
	// (optional) Domain name when used with self-hosted version of Private Captcha
	Domain string
	// (required) API key created in Private Captcha account settings
	APIKey string
	// (optional) Custom form field to read puzzle solution from (only used for VerifyRequest helper)
	FormField string
	// (optional) Custom http.Client to use with requests
	Client *http.Client
	// (optional) http status to return for failed verifications (defaults to http.StatusForbidden)
	FailedStatusCode int
}

type Client struct {
	endpoint         string
	apiKey           string
	formField        string
	failedStatusCode int
	client           *http.Client
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

	if cfg.FailedStatusCode == 0 {
		cfg.FailedStatusCode = http.StatusForbidden
	}

	return &Client{
		endpoint:         fmt.Sprintf("https://%s/verify", strings.Trim(cfg.Domain, "/")),
		apiKey:           cfg.APIKey,
		client:           cfg.Client,
		formField:        cfg.FormField,
		failedStatusCode: cfg.FailedStatusCode,
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

		return nil, seconds, retriableError{HTTPError{
			StatusCode: resp.StatusCode,
		}}
	}

	if (resp.StatusCode >= 500) ||
		(resp.StatusCode == http.StatusRequestTimeout) ||
		(resp.StatusCode == http.StatusTooEarly) {
		return nil, 0, retriableError{HTTPError{StatusCode: resp.StatusCode}}
	}

	if resp.StatusCode >= 300 {
		return nil, 0, HTTPError{StatusCode: resp.StatusCode}
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
// In case of errors, can use VerificationResponse.RequestID() for tracing.
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

	var rerr retriableError
	if (err != nil) && errors.As(err, &rerr) {
		return response, rerr.Unwrap()
	}

	return response, err
}

// VerifyRequest fetches puzzle solution from HTTP form field configured on creation and calls Verify() with defaults
func (c *Client) VerifyRequest(ctx context.Context, r *http.Request) error {
	solution := r.FormValue(c.formField)

	output, err := c.Verify(ctx, VerifyInput{Solution: solution})
	if err != nil {
		return err
	}

	if !output.Success {
		return fmt.Errorf("captcha verification failed: %v", output.Error())
	}

	return nil
}

// VerifyFunc is a basic http middleware that verifies captcha solution sent via form
func (c *Client) VerifyFunc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := c.VerifyRequest(r.Context(), r); err != nil {
			http.Error(w, http.StatusText(c.failedStatusCode), c.failedStatusCode)
			return
		}

		next.ServeHTTP(w, r)
	})
}
