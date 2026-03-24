package client

import (
	"encoding/json"
	"fmt"
	"go-car-park/internal/config"
	"go-car-park/internal/model"
	"net/http"
)

type AvailabilityClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewAvailabilityClient(cfg *config.Config) *AvailabilityClient {
	return &AvailabilityClient{
		BaseURL: cfg.ExternalAPIURL,
		HTTPClient: &http.Client{
			Timeout: cfg.APITimeout, // Always set a timeout in Go!
		},
	}
}

// FetchAvailability is your "Feign Method"
func (c *AvailabilityClient) FetchAvailability() (*model.AvailabilityResponse, error) {
	resp, err := c.HTTPClient.Get(c.BaseURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("external api error: %d", resp.StatusCode)
	}

	var result model.AvailabilityResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}
