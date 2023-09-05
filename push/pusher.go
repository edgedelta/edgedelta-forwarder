package push

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/edgedelta/edgedelta-forwarder/cfg"
)

var (
	defaultRandomizationFactor = 0.5
)

var (
	newHTTPClientFunc = func() *http.Client {
		t := &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			// MaxIdleConnsPerHost does not work as expected
			// https://github.com/golang/go/issues/13801
			// https://github.com/OJ/gobuster/issues/127
			// Improve connection re-use
			MaxIdleConns: 256,
			// Observed rare 1 in 100k connection reset by peer error with high number MaxIdleConnsPerHost
			// Most likely due to concurrent connection limit from server side per host
			// https://edgedelta.atlassian.net/browse/ED-663
			MaxIdleConnsPerHost:   128,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		}
		return &http.Client{Transport: t}
	}
)

func DoWithExpBackoffC(ctx context.Context, f func() error, initialInterval time.Duration) error {
	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.RandomizationFactor = defaultRandomizationFactor
	expBackoff.InitialInterval = initialInterval
	b := backoff.WithContext(expBackoff, ctx)
	return backoff.Retry(f, b)
}

type Pusher struct {
	endpoint      string
	retryInterval time.Duration
	pushTimeout   time.Duration
	httpClient    *http.Client
	name          string
}

func NewPusher(conf *cfg.Config) *Pusher {
	return &Pusher{
		name:          "Pusher",
		endpoint:      conf.EDEndpoint,
		retryInterval: conf.RetryInterval,
		pushTimeout:   conf.PushTimeout,
		httpClient:    newHTTPClientFunc(),
	}
}

// blocking
func (p *Pusher) Push(ctx context.Context, payload []byte) error {
	err := DoWithExpBackoffC(ctx, func() error {
		reqCtx, cancel := context.WithTimeout(ctx, p.pushTimeout)
		defer cancel()
		return p.makeHTTPRequest(reqCtx, payload)
	}, p.retryInterval)
	if err != nil {
		return fmt.Errorf("failed to push logs, err: %v", err)
	}
	return nil
}

func (p *Pusher) makeHTTPRequest(ctx context.Context, payload []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create http post request: %s, err: %v", p.endpoint, err)
	}
	req.Close = true
	req.Header.Add("Content-Type", "application/json")
	return p.sendWithCaringResponseCode(req)
}

func (p *Pusher) sendWithCaringResponseCode(req *http.Request) error {
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("%s response body read failed, err: %v", p.name, err)
		}
		body := string(bodyBytes)
		if body != "" {
			return fmt.Errorf("%s returned unexpected status code: %v response: %s", p.name, resp.StatusCode, body)
		}
		return fmt.Errorf("%s returned unexpected status code: %v", p.name, resp.StatusCode)
	}

	return nil
}
