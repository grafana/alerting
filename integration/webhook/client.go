package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type WebhookClient struct {
	c http.Client
	u *url.URL
}

func NewWebhookClient(u string) (*WebhookClient, error) {
	pu, err := url.Parse(u)
	if err != nil {
		return nil, err
	}

	return &WebhookClient{
		c: http.Client{},
		u: pu,
	}, nil
}

// GetAlerts fetches the alerts from the webhook server
func (c *WebhookClient) GetAlerts() (map[string]any, error) {
	u := c.u.ResolveReference(&url.URL{Path: "/alerts"})

	resp, err := c.c.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	res := make(map[string]any)

	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// CreateAlert creates a new alert
func (c *WebhookClient) CreateAlert(a map[string]any) error {
	u := c.u.ResolveReference(&url.URL{Path: "/alert"})

	d, err := json.Marshal(a)
	if err != nil {
		return err
	}

	resp, err := c.c.Post(u.String(), "application/json", bytes.NewReader(d))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

//func main() {
//	wc, err := NewWebhookClient("http://localhost:8080")
//	if err != nil {
//		fmt.Printf("error: %v\n", err)
//		os.Exit(1)
//	}
//
//	as, err := wc.GetAlerts()
//	if err != nil {
//		fmt.Printf("error: %v\n", err)
//		os.Exit(1)
//	}
//	fmt.Printf("as = %v\n", as)
//
//	err = wc.CreateAlert(map[string]any{"test": "abcd"})
//	if err != nil {
//		fmt.Printf("error: %v\n", err)
//		os.Exit(1)
//	}
//
//	as, err = wc.GetAlerts()
//	if err != nil {
//		fmt.Printf("error: %v\n", err)
//		os.Exit(1)
//	}
//	fmt.Printf("as = %v\n", as)
//}
