package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/internetarchive/doppelganger/pkg/server/models"
)

type Client struct {
	BaseURL      string
	addRecordURL string
	HTTPClient   *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL:      baseURL,
		addRecordURL: fmt.Sprintf("%s/api/records/", baseURL),
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second, // Set the timeout to 30 seconds
		},
	}
}

func (c *Client) GetRecord(ID string) (*models.Record, error) {
	url := fmt.Sprintf("%s/api/records/%s", c.BaseURL, ID)
	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get record: %s", resp.Status)
	}

	var record models.Record
	if err := json.NewDecoder(resp.Body).Decode(&record); err != nil {
		return nil, err
	}

	return &record, nil
}

func (c *Client) AddRecords(records ...*models.Record) error {
	body, err := json.Marshal(records)
	if err != nil {
		return err
	}

	resp, err := c.HTTPClient.Post(c.addRecordURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to add record: %s", resp.Status)
	}

	return nil
}
