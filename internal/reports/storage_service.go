package reports

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const reportsBucket = "reports"

// StorageClient wraps Supabase Storage REST calls.
type StorageClient struct {
	baseURL        string
	serviceRoleKey string
	httpClient     *http.Client
}

func NewStorageClient(supabaseURL, serviceRoleKey string) *StorageClient {
	return &StorageClient{
		baseURL:        strings.TrimRight(supabaseURL, "/"),
		serviceRoleKey: serviceRoleKey,
		httpClient:     &http.Client{Timeout: 60 * time.Second},
	}
}

// Upload uploads bytes to the reports bucket at storagePath.
func (s *StorageClient) Upload(storagePath string, data []byte, contentType string) error {
	url := fmt.Sprintf("%s/storage/v1/object/%s/%s", s.baseURL, reportsBucket, storagePath)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("storage upload request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.serviceRoleKey)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("x-upsert", "true")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("storage upload: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("storage upload status %d: %s", resp.StatusCode, body)
	}
	return nil
}

// SignedURL generates a signed download URL valid for expiresIn seconds.
// Mirrors the Supabase JS client: extract the token from the response and
// construct the URL ourselves so the format is always correct regardless of
// which Supabase Storage version is running.
func (s *StorageClient) SignedURL(storagePath string, expiresIn int) (string, time.Time, error) {
	endpoint := fmt.Sprintf("%s/storage/v1/object/sign/%s/%s", s.baseURL, reportsBucket, storagePath)
	body, _ := json.Marshal(map[string]int{"expiresIn": expiresIn})

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("signed url request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.serviceRoleKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("signed url: %w", err)
	}
	defer resp.Body.Close()

	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", time.Time{}, fmt.Errorf("signed url status %d: %s", resp.StatusCode, rb)
	}

	// Supabase Storage returns { "signedURL": "...", "token": "eyJ...", "path": "..." }
	// The signedURL field format varies across versions (/object/sign/... vs /storage/v1/object/sign/...).
	// Always use the token to build the URL ourselves — same approach as the JS client.
	var result struct {
		Token     string `json:"token"`
		SignedURL string `json:"signedURL"` // fallback if token absent
	}
	if err := json.Unmarshal(rb, &result); err != nil {
		return "", time.Time{}, fmt.Errorf("signed url decode: %w", err)
	}

	var signedURL string
	if result.Token != "" {
		// Construct deterministically: baseURL/storage/v1/object/sign/bucket/path?token=JWT
		signedURL = fmt.Sprintf("%s/storage/v1/object/sign/%s/%s?token=%s",
			s.baseURL, reportsBucket, storagePath, result.Token)
	} else if result.SignedURL != "" {
		if strings.HasPrefix(result.SignedURL, "/") {
			// Ensure /storage/v1 prefix is present
			if !strings.HasPrefix(result.SignedURL, "/storage/") {
				signedURL = s.baseURL + "/storage/v1" + result.SignedURL
			} else {
				signedURL = s.baseURL + result.SignedURL
			}
		} else {
			signedURL = result.SignedURL
		}
	} else {
		return "", time.Time{}, fmt.Errorf("signed url: no token or signedURL in response: %s", rb)
	}

	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)
	return signedURL, expiresAt, nil
}

// deleteObject sends a DELETE to Supabase Storage (best-effort, used for cleanup).
func (s *StorageClient) deleteObject(storagePath string) error {
	url := fmt.Sprintf("%s/storage/v1/object/%s/%s", s.baseURL, reportsBucket, storagePath)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.serviceRoleKey)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
