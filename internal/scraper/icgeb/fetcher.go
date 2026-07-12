package icgeb

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"
)

func fetchWithUA(ctx context.Context, url, ua string) (string, error) {
	const maxRetries = 6
	delay := time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", ua)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}

		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			resp.Body.Close()
			if attempt == maxRetries {
				return "", fmt.Errorf("request failed: %d %s", resp.StatusCode, resp.Status)
			}
			jitter := time.Duration(rand.Intn(500)) * time.Millisecond
			select {
			case <-time.After(delay + jitter):
			case <-ctx.Done():
				return "", ctx.Err()
			}
			delay *= 2
			if delay > 10*time.Second {
				delay = 10 * time.Second
			}
			continue
		}

		if resp.StatusCode == 404 {
			resp.Body.Close()
			return "", nil
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			return "", fmt.Errorf("request failed: %d %s", resp.StatusCode, resp.Status)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", err
		}
		return string(body), nil
	}

	return "", fmt.Errorf("request failed after max retries")
}
