package service

import (
	"fmt"
	"net"
	"sort"
	"strings"
)

type PublicDomainResolveInput struct {
	ProjectID            string
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
	Status     string
	Reason     string
}

type PublicDomainResolver struct {
	instances InstanceStore
	clusters  ClusterStore
	domains   *ProjectDomainService
	routing   *RoutingService
}

func NewPublicDomainResolver(
	instances InstanceStore,
	clusters ClusterStore,
	domains *ProjectDomainService,
	routing *RoutingService,
) *PublicDomainResolver {
	return &PublicDomainResolver{
		instances: instances,
		clusters:  clusters,
		domains:   domains,
		routing:   routing,
	}
}

func (r *PublicDomainResolver) Resolve(input PublicDomainResolveInput) PublicDomainResult {
	if r == nil {
		return PublicDomainResult{
			PublicURLs: []string{},
			Reason:     "Không thể xác định domain công khai vì thiếu public domain resolver.",
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
	if strings.TrimSpace(input.ProjectID) == "" {
		return PublicDomainResult{
			PublicURLs: []string{},
			Reason:     "Thiếu project_id nên không thể resolve managed domain.",
		}
	}

	publicIPByService := make(map[string]string, len(publicServices))
	for _, service := range publicServices {
		publicIP := r.resolveTargetPublicIP(strings.TrimSpace(input.TargetKind), strings.TrimSpace(input.TargetID), service.Name, input.PlacementAssignments)
		if publicIP == "" || net.ParseIP(publicIP) == nil || isPrivateIP(publicIP) {
			continue
		}
		publicIPByService[service.Name] = publicIP
	}
	targetPublicIP := ""
	for _, service := range publicServices {
		if publicIP := strings.TrimSpace(publicIPByService[service.Name]); publicIP != "" {
			targetPublicIP = publicIP
			break
		}
	}
	if targetPublicIP == "" {
		return PublicDomainResult{
			Domains:    []PublicDomainRecord{},
			PublicURLs: []string{},
			Reason:     "Target chưa có public IP hợp lệ để cấp managed domain.",
		}
	}
	if r.domains == nil {
		return PublicDomainResult{
			PublicURLs: []string{},
			Reason:     "Không thể xác định managed domain vì thiếu project domain service.",
		}
	}

	managedDomain, err := r.domains.EnsureManagedDomain(
		input.ProjectID,
		input.ProjectSlug,
		strings.TrimSpace(input.TargetKind),
		strings.TrimSpace(input.TargetID),
		targetPublicIP,
	)
	if err != nil {
		return PublicDomainResult{
			PublicURLs: []string{},
			Reason:     fmt.Sprintf("Không thể cấp managed domain cho project: %v", err),
		}
	}
	sharedDomain := strings.TrimSpace(managedDomain.Hostname)
	if sharedDomain == "" {
		return PublicDomainResult{
			PublicURLs: []string{},
			Reason:     "Managed domain chưa có hostname hợp lệ.",
		}
	}

	routing := RoutingPolicyRecord{
		SharedDomain: sharedDomain,
		Routes:       []RoutingRouteRecord{},
	}
	if r.routing != nil {
		resolved, err := r.routing.ResolveEffectiveRouting(input.ProjectID, publicServices, sharedDomain)
		if err == nil {
			routing = resolved
		}
	}
	domains := buildManagedPublicDomainRecords(routing, publicServices)
	publicURLs := collectPublicURLsFromDomains(domains)
	return PublicDomainResult{
		Domains:    domains,
		PublicURLs: publicURLs,
		Status:     managedDomain.Status,
		Reason:     strings.TrimSpace(managedDomain.StatusReason),
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

func buildManagedPublicDomainRecords(routing RoutingPolicyRecord, services []BlueprintServiceContractRecord) []PublicDomainRecord {
	sharedDomain := strings.TrimSpace(routing.SharedDomain)
	if sharedDomain == "" {
		return []PublicDomainRecord{}
	}

	serviceURLIndex := make(map[string]string, len(routing.Routes))
	for _, route := range routing.Routes {
		serviceName := strings.TrimSpace(route.Service)
		if serviceName == "" {
			continue
		}
		if _, exists := serviceURLIndex[serviceName]; exists {
			continue
		}
		serviceURLIndex[serviceName] = managedProjectDomainURL(sharedDomain) + normalizePublicURLPath(route.Path)
	}

	out := make([]PublicDomainRecord, 0, len(services))
	for _, service := range services {
		url := strings.TrimSpace(serviceURLIndex[service.Name])
		if url == "" {
			url = managedProjectDomainURL(sharedDomain)
		}
		out = append(out, PublicDomainRecord{
			ServiceName:  service.Name,
			PrimaryHost:  sharedDomain,
			FallbackHost: sharedDomain,
			PrimaryURL:   url,
			FallbackURL:  url,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].ServiceName < out[j].ServiceName
	})
	return out
}

func normalizePublicURLPath(path string) string {
	trimmed := strings.TrimSpace(path)
	switch trimmed {
	case "", "/":
		return ""
	default:
		if !strings.HasPrefix(trimmed, "/") {
			trimmed = "/" + trimmed
		}
		return trimmed
	}
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
