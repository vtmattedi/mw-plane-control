package mwmqtt

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const defaultMaxResponseBytes int64 = 2 * 1024 * 1024

var (
	ErrResponseTooLarge = errors.New("MW MQTT response too large")
	safeRequestID       = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)
)

type Config struct {
	BaseURL          string
	Host             string
	HTTPTimeout      time.Duration
	MaxResponseBytes int64
}

type Client struct {
	baseURL          string
	host             string
	maxResponseBytes int64
	httpClient       *http.Client
}

type Response struct {
	StatusCode  int
	Body        []byte
	ContentType string
	RequestID   string
}

func New(config Config) (*Client, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("MW MQTT base URL is required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.Host == "" {
		return nil, fmt.Errorf("MW MQTT base URL must be absolute http/https")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		(parsed.Path != "" && parsed.Path != "/") {
		return nil, fmt.Errorf(
			"MW MQTT base URL must not contain credentials, path, query, or fragment",
		)
	}

	host := strings.TrimSpace(config.Host)
	if host == "" {
		host = parsed.Host
	}
	if strings.Contains(host, "://") ||
		strings.ContainsAny(host, " \t\r\n/\\?#@") {
		return nil, fmt.Errorf("MW MQTT logical Host is invalid")
	}

	if config.HTTPTimeout <= 0 {
		return nil, fmt.Errorf("MW MQTT HTTP timeout must be positive")
	}
	if config.MaxResponseBytes <= 0 {
		config.MaxResponseBytes = defaultMaxResponseBytes
	}

	return &Client{
		baseURL:          baseURL,
		host:             host,
		maxResponseBytes: config.MaxResponseBytes,
		httpClient: &http.Client{
			Timeout: config.HTTPTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func (c *Client) Do(
	ctx context.Context,
	method string,
	downstreamPath string,
	rawQuery string,
	bearerToken string,
	contentType string,
	accept string,
	requestID string,
	body []byte,
) (Response, error) {
	if c == nil {
		return Response{}, fmt.Errorf("MW MQTT client is disabled")
	}
	bearerToken = strings.TrimSpace(bearerToken)
	if bearerToken == "" {
		return Response{}, fmt.Errorf("delegated bearer token is required")
	}
	if !strings.HasPrefix(downstreamPath, "/admin/api/") &&
		downstreamPath != "/admin/api" {
		return Response{}, fmt.Errorf("invalid MW MQTT downstream path")
	}
	if strings.Contains(downstreamPath, "\\") ||
		strings.Contains(downstreamPath, "..") ||
		strings.Contains(downstreamPath, "//") {
		return Response{}, fmt.Errorf("invalid MW MQTT downstream path")
	}

	target := c.baseURL + downstreamPath
	if strings.TrimSpace(rawQuery) != "" {
		target += "?" + rawQuery
	}

	req, err := http.NewRequestWithContext(
		ctx,
		method,
		target,
		bytes.NewReader(body),
	)
	if err != nil {
		return Response{}, fmt.Errorf("build MW MQTT request: %w", err)
	}
	req.Host = c.host
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	if value := strings.TrimSpace(contentType); value != "" {
		req.Header.Set("Content-Type", value)
	}
	if value := strings.TrimSpace(accept); value != "" {
		req.Header.Set("Accept", value)
	} else {
		req.Header.Set("Accept", "application/json")
	}
	if safeRequestID.MatchString(strings.TrimSpace(requestID)) {
		req.Header.Set("X-Request-ID", strings.TrimSpace(requestID))
	}

	response, err := c.httpClient.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("MW MQTT request failed: %w", err)
	}
	defer response.Body.Close()

	limited := io.LimitReader(response.Body, c.maxResponseBytes+1)
	payload, err := io.ReadAll(limited)
	if err != nil {
		return Response{}, fmt.Errorf("read MW MQTT response: %w", err)
	}
	if int64(len(payload)) > c.maxResponseBytes {
		return Response{}, ErrResponseTooLarge
	}

	return Response{
		StatusCode:  response.StatusCode,
		Body:        payload,
		ContentType: response.Header.Get("Content-Type"),
		RequestID:   response.Header.Get("X-Request-ID"),
	}, nil
}
