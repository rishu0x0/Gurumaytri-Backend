package msg91

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	baseURL = "https://control.msg91.com/api/v5/widget"
)

type Client struct {
	authToken  string
	widgetID   string
	httpClient *http.Client
}

func New(authToken, widgetID string) *Client {
	return &Client{
		authToken: authToken,
		widgetID:  widgetID,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type sendOTPRequest struct {
	WidgetID   string `json:"widgetId"`
	Identifier string `json:"identifier"`
}

type SendOTPResponse struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	// Message contains reqId on success
}

// SendOTP sends an OTP to the given mobile number (must include country code, e.g. 91XXXXXXXXXX).
// Returns the reqId from MSG91 on success.
func (c *Client) SendOTP(ctx context.Context, mobile string) (string, error) {
	body := sendOTPRequest{
		WidgetID:   c.widgetID,
		Identifier: mobile,
	}
	resp, err := c.post(ctx, "/sendOTP", body)
	if err != nil {
		return "", err
	}
	if resp.Type != "success" {
		return "", fmt.Errorf("msg91 sendOTP failed: %s", resp.Message)
	}
	return resp.Message, nil
}

type verifyOTPRequest struct {
	WidgetID string `json:"widgetId"`
	ReqID    string `json:"reqId"`
	OTP      string `json:"otp"`
}

// VerifyOTP verifies the OTP entered by the user.
// Returns nil on success, error on failure (wrong OTP, expired, etc.).
func (c *Client) VerifyOTP(ctx context.Context, reqID, otp string) error {
	body := verifyOTPRequest{
		WidgetID: c.widgetID,
		ReqID:    reqID,
		OTP:      otp,
	}
	resp, err := c.post(ctx, "/verifyOTP", body)
	if err != nil {
		return err
	}
	if resp.Type != "success" {
		return fmt.Errorf("otp_invalid")
	}
	return nil
}

type retryOTPRequest struct {
	WidgetID     string `json:"widgetId"`
	ReqID        string `json:"reqId"`
	RetryChannel int    `json:"retryChannel"`
}

// RetryOTP retries OTP delivery. retryChannel: 11=SMS, 4=VOICE, 3=EMAIL, 12=WHATSAPP
func (c *Client) RetryOTP(ctx context.Context, reqID string, retryChannel int) error {
	body := retryOTPRequest{
		WidgetID:     c.widgetID,
		ReqID:        reqID,
		RetryChannel: retryChannel,
	}
	resp, err := c.post(ctx, "/retryOTP", body)
	if err != nil {
		return err
	}
	if resp.Type != "success" {
		return fmt.Errorf("msg91 retryOTP failed: %s", resp.Message)
	}
	return nil
}

func (c *Client) post(ctx context.Context, path string, payload interface{}) (*SendOTPResponse, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("authkey", c.authToken)

	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request to MSG91: %w", err)
	}
	defer httpResp.Body.Close()

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read MSG91 response: %w", err)
	}

	var result SendOTPResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("parse MSG91 response: %w", err)
	}
	return &result, nil
}
