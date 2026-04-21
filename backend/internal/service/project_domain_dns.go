package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"lazyops-server/internal/config"
)

type ProjectDomainDNSUpsertRequest struct {
	Hostname         string
	IPv4             string
	Proxied          bool
	ExistingRecordID string
}

type ProjectDomainDNSSyncResult struct {
	RecordID string
}

type ProjectDomainDNSClient interface {
	UpsertARecord(context.Context, ProjectDomainDNSUpsertRequest) (ProjectDomainDNSSyncResult, error)
}

type NoopProjectDomainDNSClient struct{}

func (c *NoopProjectDomainDNSClient) UpsertARecord(_ context.Context, req ProjectDomainDNSUpsertRequest) (ProjectDomainDNSSyncResult, error) {
	recordID := strings.TrimSpace(req.ExistingRecordID)
	if recordID == "" {
		recordID = "dns_noop"
	}
	return ProjectDomainDNSSyncResult{RecordID: recordID}, nil
}

type CloudflareDNSClient struct {
	baseURL string
	zoneID  string
	token   string
	client  *http.Client
}

func NewProjectDomainDNSClient(cfg config.PublicDomainConfig) ProjectDomainDNSClient {
	if strings.TrimSpace(strings.ToLower(cfg.Provider)) != "cloudflare" {
		return &NoopProjectDomainDNSClient{}
	}
	if strings.TrimSpace(cfg.CloudflareZoneID) == "" || strings.TrimSpace(cfg.CloudflareAPIToken) == "" {
		return &NoopProjectDomainDNSClient{}
	}
	return &CloudflareDNSClient{
		baseURL: "https://api.cloudflare.com/client/v4",
		zoneID:  strings.TrimSpace(cfg.CloudflareZoneID),
		token:   strings.TrimSpace(cfg.CloudflareAPIToken),
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *CloudflareDNSClient) WithBaseURL(baseURL string) *CloudflareDNSClient {
	if c == nil {
		return c
	}
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmed != "" {
		c.baseURL = trimmed
	}
	return c
}

func (c *CloudflareDNSClient) UpsertARecord(ctx context.Context, req ProjectDomainDNSUpsertRequest) (ProjectDomainDNSSyncResult, error) {
	if c == nil {
		return ProjectDomainDNSSyncResult{}, fmt.Errorf("cloudflare dns client is nil")
	}
	payload := map[string]any{
		"type":    "A",
		"name":    strings.TrimSpace(req.Hostname),
		"content": strings.TrimSpace(req.IPv4),
		"proxied": req.Proxied,
		"ttl":     1,
	}
	method := http.MethodPost
	endpoint := fmt.Sprintf("%s/zones/%s/dns_records", c.baseURL, c.zoneID)
	if strings.TrimSpace(req.ExistingRecordID) != "" {
		method = http.MethodPut
		endpoint = fmt.Sprintf("%s/zones/%s/dns_records/%s", c.baseURL, c.zoneID, strings.TrimSpace(req.ExistingRecordID))
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return ProjectDomainDNSSyncResult{}, fmt.Errorf("marshal cloudflare dns payload: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return ProjectDomainDNSSyncResult{}, fmt.Errorf("build cloudflare dns request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return ProjectDomainDNSSyncResult{}, fmt.Errorf("cloudflare dns request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ProjectDomainDNSSyncResult{}, fmt.Errorf("read cloudflare dns response: %w", err)
	}

	var envelope struct {
		Success bool `json:"success"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
		Result struct {
			ID string `json:"id"`
		} `json:"result"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return ProjectDomainDNSSyncResult{}, fmt.Errorf("decode cloudflare dns response: %w", err)
	}
	if resp.StatusCode >= 300 || !envelope.Success {
		reason := fmt.Sprintf("cloudflare dns upsert failed with status %d", resp.StatusCode)
		if len(envelope.Errors) > 0 && strings.TrimSpace(envelope.Errors[0].Message) != "" {
			reason = envelope.Errors[0].Message
		}
		return ProjectDomainDNSSyncResult{}, errors.New(reason)
	}
	return ProjectDomainDNSSyncResult{RecordID: strings.TrimSpace(envelope.Result.ID)}, nil
}
