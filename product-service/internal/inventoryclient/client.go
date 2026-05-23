package inventoryclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func New(baseURL string) *Client {
	return &Client{baseURL: baseURL, httpClient: &http.Client{Timeout: 5 * time.Second}}
}

func (c *Client) SetStock(ctx context.Context, productID string, qty int) error {
	body, _ := json.Marshal(map[string]int{"quantity": qty})
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+"/api/v1/inventory/"+productID, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("inventory set stock status %d", res.StatusCode)
	}
	return nil
}

func (c *Client) GetStock(ctx context.Context, productID string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/inventory/"+productID, nil)
	if err != nil {
		return 0, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return 0, nil
	}
	if res.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("inventory get stock status %d", res.StatusCode)
	}
	var out struct {
		Data struct {
			Quantity int `json:"quantity"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return 0, err
	}
	return out.Data.Quantity, nil
}
