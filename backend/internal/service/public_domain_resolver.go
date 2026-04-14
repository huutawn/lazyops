package service

import (
	"fmt"
	"net"
	"sort"
	"strings"
)

type PublicDomainResolveInput struct {
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
}

func NewPublicDomainResolver(instances InstanceStore) *PublicDomainResolver {
	return &PublicDomainResolver{instances: instances}
}

func (r *PublicDomainResolver) Resolve(input PublicDomainResolveInput) PublicDomainResult {
	if r == nil || r.instances == nil {
		return PublicDomainResult{
			PublicURLs: []string{},
			Reason:     "Không thể xác định domain công khai vì thiếu instance store.",
		}
	}

	if strings.TrimSpace(input.RuntimeMode) != bootstrapModeStandalone {
		return PublicDomainResult{
			PublicURLs: []string{},
			Reason:     "Domain công khai theo public IP hiện chỉ hỗ trợ cho standalone instance.",
		}
	}

	if strings.TrimSpace(input.TargetKind) != "instance" {
		return PublicDomainResult{
			PublicURLs: []string{},
			Reason:     "Target hiện tại không phải instance nên chưa có magic domain theo public IP.",
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
		targetID := placementTargetIDForService(service.Name, input.PlacementAssignments)
		if targetID == "" {
			targetID = strings.TrimSpace(input.TargetID)
		}
		if targetID == "" {
			continue
		}

		instance, err := r.instances.GetByID(targetID)
		if err != nil || instance == nil || instance.PublicIP == nil {
			continue
		}

		publicIP := strings.TrimSpace(*instance.PublicIP)
		if publicIP == "" || net.ParseIP(publicIP) == nil || isPrivateIP(publicIP) {
			continue
		}

		dashedIP := strings.ReplaceAll(publicIP, ".", "-")
		primaryHost := fmt.Sprintf("%s.%s.%s", service.Name, dashedIP, MagicDomainProviderSSLIP)
		fallbackHost := fmt.Sprintf("%s.%s.%s", service.Name, dashedIP, MagicDomainProviderNipIO)

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
