package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type StripeClient interface {
	CreateCustomer(ctx context.Context, email, userID string) (string, error)
	CreateCheckoutSession(ctx context.Context, params CheckoutSessionParams) (string, error)
	CreatePortalSession(ctx context.Context, customerID, returnURL string) (string, error)
}

type stripeHTTPClient struct {
	secretKey string
	client    *http.Client
	apiBase   string
}

func NewStripeHTTPClient(secretKey string) StripeClient {
	return &stripeHTTPClient{
		secretKey: secretKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiBase: "https://api.stripe.com",
	}
}

type CheckoutSessionMode string

const (
	CheckoutSessionModeSubscription CheckoutSessionMode = "subscription"
	CheckoutSessionModePayment      CheckoutSessionMode = "payment"
)

type CheckoutSessionParams struct {
	Mode       CheckoutSessionMode
	CustomerID string
	PriceID    string
	SuccessURL string
	CancelURL  string
	Metadata   map[string]string
}

func (c *stripeHTTPClient) CreateCustomer(ctx context.Context, email, userID string) (string, error) {
	form := url.Values{}
	form.Set("email", email)
	form.Set("metadata[user_id]", userID)
	form.Set("metadata[app]", "yearofbingo")

	var resp struct {
		ID string `json:"id"`
	}
	if err := c.postForm(ctx, "/v1/customers", form, &resp); err != nil {
		return "", err
	}
	if resp.ID == "" {
		return "", fmt.Errorf("stripe: missing customer id in response")
	}
	return resp.ID, nil
}

func (c *stripeHTTPClient) CreateCheckoutSession(ctx context.Context, params CheckoutSessionParams) (string, error) {
	form := url.Values{}
	form.Set("mode", string(params.Mode))
	form.Set("customer", params.CustomerID)
	form.Set("success_url", params.SuccessURL)
	form.Set("cancel_url", params.CancelURL)

	// Stripe Tax
	form.Set("automatic_tax[enabled]", "true")
	form.Set("billing_address_collection", "required")
	form.Set("customer_update[address]", "auto")

	form.Set("line_items[0][price]", params.PriceID)
	form.Set("line_items[0][quantity]", "1")

	for k, v := range params.Metadata {
		form.Set("metadata["+k+"]", v)
	}

	var resp struct {
		URL string `json:"url"`
	}
	if err := c.postForm(ctx, "/v1/checkout/sessions", form, &resp); err != nil {
		return "", err
	}
	if resp.URL == "" {
		return "", fmt.Errorf("stripe: missing checkout url in response")
	}
	return resp.URL, nil
}

func (c *stripeHTTPClient) CreatePortalSession(ctx context.Context, customerID, returnURL string) (string, error) {
	form := url.Values{}
	form.Set("customer", customerID)
	form.Set("return_url", returnURL)

	var resp struct {
		URL string `json:"url"`
	}
	if err := c.postForm(ctx, "/v1/billing_portal/sessions", form, &resp); err != nil {
		return "", err
	}
	if resp.URL == "" {
		return "", fmt.Errorf("stripe: missing portal url in response")
	}
	return resp.URL, nil
}

func (c *stripeHTTPClient) postForm(ctx context.Context, path string, form url.Values, out any) error {
	body := strings.NewReader(form.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiBase+path, body)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.secretKey, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // close errors are not actionable here

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return stripeAPIError(resp.StatusCode, respBytes)
	}

	if err := json.Unmarshal(respBytes, out); err != nil {
		return fmt.Errorf("stripe: decode response: %w", err)
	}
	return nil
}

func stripeAPIError(status int, respBytes []byte) error {
	var e struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	_ = json.Unmarshal(respBytes, &e)
	if msg := strings.TrimSpace(e.Error.Message); msg != "" {
		return fmt.Errorf("stripe: status %d: %s", status, msg)
	}
	return fmt.Errorf("stripe: status %d", status)
}
