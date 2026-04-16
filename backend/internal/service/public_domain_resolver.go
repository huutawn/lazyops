package service

import (
	"fmt"
	"net"
	"sort"
	"strings"
)

type PublicDomainResolveInput struct {
	ProjectSlug          string
	RuntimeMode          string
	TargetKind           string
	TargetID             string
	Services             []BlueprintServiceContractRecord
	PlacementAssignments []PlacementAssignmentRecord
}

type PublicDomainRecord struct {
	ServiceName  string
	PrimaryHost  string
	FallbackHost string
	PrimaryURL   string
	FallbackURL  string
}

type PublicDomainResult struct {
	Domains    []PublicDomainRecord
	PublicURLs []string
	Reason     string
}

type PublicDomainResolver struct {
	instances InstanceStore
	clusters  ClusterStore
}

func NewPublicDomainResolver(instances InstanceStore, clusters ClusterStore) *PublicDomainResolver {
	return &PublicDomainResolver{
		instances: instances,
		clusters:  clusters,
	}
}

func (r *PublicDomainResolver) Resolve(input PublicDomainResolveInput) PublicDomainResult {
	if r == nil {
		return PublicDomainResult{
			PublicURLs: []string{},
			Reason:     "Không thể xác định domain công khai vì thiếu public domain resolver.",
		}
	}

	runtimeMode := strings.TrimSpace(input.RuntimeMode)
	targetKind := strings.TrimSpace(input.TargetKind)
	if runtimeMode != bootstrapModeStandalone && runtimeMode != bootstrapModeDistributedK3s {
		return PublicDomainResult{
			PublicURLs: []string{},
			Reason:     "Magic domain theo public IP hiện chỉ hỗ trợ cho standalone instance hoặc distributed-k3s.",
		}
	}

	publicServices := make([]BlueprintServiceContractRecord, 0)
	for _, service := range input.Services {
		if service.Public {
			publicServices = append(publicServices, service)
		}
	}
	if len(publicServices) == 0 {
		return PublicDomainResult{
			PublicURLs: []string{},
			Reason:     "Revision này không có dịch vụ public.",
		}
	}

	domains := make([]PublicDomainRecord, 0, len(publicServices))
	for _, service := range publicServices {
		publicIP := r.resolveTargetPublicIP(targetKind, strings.TrimSpace(input.TargetID), service.Name, input.PlacementAssignments)
		if publicIP == "" || net.ParseIP(publicIP) == nil || isPrivateIP(publicIP) {
			continue
		}

		dashedIP := strings.ReplaceAll(publicIP, ".", "-")
		projectToken := strings.TrimSpace(input.ProjectSlug)
		if projectToken == "" {
			projectToken = strings.TrimSpace(input.TargetID)
		}
		if strings.TrimSpace(projectToken) == "" {
			projectToken = "cluster"
		}
		projectToken = sanitizeDomainLabel(projectToken)
		primaryHost := fmt.Sprintf("%s.%s.%s.%s", service.Name, projectToken, dashedIP, MagicDomainProviderSSLIP)
		fallbackHost := fmt.Sprintf("%s.%s.%s.%s", service.Name, projectToken, publicIP, MagicDomainProviderNipIO)

		domains = append(domains, PublicDomainRecord{
			ServiceName:  service.Name,
			PrimaryHost:  primaryHost,
			FallbackHost: fallbackHost,
			PrimaryURL:   "https://" + primaryHost,
			FallbackURL:  "https://" + fallbackHost,
		})
	}

	sort.Slice(domains, func(i, j int) bool {
		return domains[i].ServiceName < domains[j].ServiceName
	})

	publicURLs := collectPublicURLsFromDomains(domains)
	if len(publicURLs) > 0 {
		return PublicDomainResult{
			Domains:    domains,
			PublicURLs: publicURLs,
		}
	}

	return PublicDomainResult{
		Domains:    []PublicDomainRecord{},
		PublicURLs: []string{},
		Reason:     "Target chưa có public IP hợp lệ để cấp magic domain.",
	}
}

func (r *PublicDomainResolver) resolveTargetPublicIP(targetKind, targetID, serviceName string, assignments []PlacementAssignmentRecord) string {
	targetID = strings.TrimSpace(targetID)
	switch strings.TrimSpace(targetKind) {
	case "instance":
		if r.instances == nil {
			return ""
		}
		placementID := placementTargetIDForService(serviceName, assignments)
		if placementID != "" {
			targetID = placementID
		}
		if targetID == "" {
			return ""
		}
		instance, err := r.instances.GetByID(targetID)
		if err != nil || instance == nil || instance.PublicIP == nil {
			return ""
		}
		return strings.TrimSpace(*instance.PublicIP)
	case "cluster":
		if r.clusters == nil || targetID == "" {
			return ""
		}
		cluster, err := r.clusters.GetByID(targetID)
		if err != nil || cluster == nil || cluster.PublicIP == nil {
			return ""
		}
		return strings.TrimSpace(*cluster.PublicIP)
	default:
		return ""
	}
}

func placementTargetIDForService(serviceName string, assignments []PlacementAssignmentRecord) string {
	for _, assignment := range assignments {
		if assignment.ServiceName == serviceName {
			return strings.TrimSpace(assignment.TargetID)
		}
	}
	return ""
}

func collectPublicURLsFromDomains(domains []PublicDomainRecord) []string {
	if len(domains) == 0 {
		return []string{}
	}

	seen := make(map[string]struct{}, len(domains)*2)
	urls := make([]string, 0, len(domains)*2)
	for _, domain := range domains {
		for _, candidate := range []string{domain.PrimaryURL, domain.FallbackURL} {
			candidate = strings.TrimSpace(candidate)
			if candidate == "" {
				continue
			}
			if _, exists := seen[candidate]; exists {
				continue
			}
			seen[candidate] = struct{}{}
			urls = append(urls, candidate)
		}
	}
	return urls
}

func sanitizeDomainLabel(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return "app"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if b.Len() > 0 && !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func toPublicDomainPayloads(domains []PublicDomainRecord) []map[string]any {
	if len(domains) == 0 {
		return nil
	}

	payloads := make([]map[string]any, 0, len(domains))
	for _, domain := range domains {
		payloads = append(payloads, map[string]any{
			"service_name":  domain.ServiceName,
			"primary_host":  domain.PrimaryHost,
			"fallback_host": domain.FallbackHost,
			"primary_url":   domain.PrimaryURL,
			"fallback_url":  domain.FallbackURL,
		})
	}
	return payloads
}
