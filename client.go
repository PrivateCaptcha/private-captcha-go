package privatecaptcha

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

var (
	headerApiKey   = http.CanonicalHeaderKey("X-Api-Key")
	errEmptyAPIKey = errors.New("API key is empty")
)

const (
	GlobalDomain = "api.privatecaptcha.com"
	EUDomain     = "api.eu.privatecaptcha.com"
)

type Configuration struct {
	Domain string
	APIKey string
	Client *http.Client
}

type client struct {
	endpoint string
	apiKey   string
	client   *http.Client
}

func NewClient(cfg Configuration) (*client, error) {
	if len(cfg.APIKey) == 0 {
		return nil, errEmptyAPIKey
	}

	if len(cfg.Domain) == 0 {
		cfg.Domain = GlobalDomain
	}

	if cfg.Client == nil {
		cfg.Client = http.DefaultClient
	}

	return &client{
		endpoint: fmt.Sprintf("https://%s/verify", strings.Trim(cfg.Domain, "/")),
		apiKey:   cfg.APIKey,
		client:   cfg.Client,
	}, nil
}

func (c *client) Verify(ctx context.Context, solution string) (*VerificationResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, strings.NewReader(solution))
	if err != nil {
		return nil, err
	}

	req.Header.Set(headerApiKey, c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	response := &VerificationResponse{}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return response, nil
}
