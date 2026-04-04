package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"lazyops-server/internal/models"
	"lazyops-server/pkg/utils"
)

const (
	DomainKindMagic    = "magic"
	DomainKindCustom   = "custom"
	DomainKindInternal = "internal"

	MagicDomainProviderSSLIP = "sslip.io"
	MagicDomainProviderNipIO = "nip.io"

	GatewayConfigStatusPending      = "pending"
	GatewayConfigStatusDispatched   = "dispatched"
	GatewayConfigStatusAcknowledged = "acknowledged"
	GatewayConfigStatusFailed       = "failed"

	PublicRouteStatusActive   = "active"
	PublicRouteStatusInactive = "inactive"
	PublicRouteStatusDraining = "draining"

	ReleaseStatusDeployed   = "deployed"
	ReleaseStatusRolledBack = "rolled_back"
	ReleaseStatusFailed     = "failed"
	ReleaseStatusPromoted   = "promoted"
)

var (
	ErrInvalidMagicDomainIP = errors.New("invalid public IP for magic domain")
	ErrInvalidDomain        = errors.New("invalid domain")
	ErrReleaseNotFound      = errors.New("release not found")
)

type GatewayConfigService struct {
	revisions      DesiredStateRevisionStore
	deployments    DeploymentStore
	bindings       DeploymentBindingStore
	routes         PublicRouteStore
	gatewayIntents GatewayConfigIntentStore
	releases       ReleaseHistoryStore
}

type PublicRouteStore interface {
	Create(route *models.PublicRoute) error
	ListByProject(projectID string) ([]models.PublicRoute, error)
	ListByDeployment(projectID, deploymentID string) ([]models.PublicRoute, error)
	UpdateStatus(routeID, status string, at time.Time) error
}

type GatewayConfigIntentStore interface {
	Create(intent *models.GatewayConfigIntent) error
	GetByIDForProject(projectID, intentID string) (*models.GatewayConfigIntent, error)
	ListByDeployment(projectID, deploymentID string) ([]models.GatewayConfigIntent, error)
	UpdateStatus(intentID, status string, at time.Time) error
}

type ReleaseHistoryStore interface {
	Create(record *models.ReleaseHistory) error
	ListByProject(projectID string, limit int) ([]models.ReleaseHistory, error)
	GetByIDForProject(projectID, recordID string) (*models.ReleaseHistory, error)
}

func NewGatewayConfigService(
	revisions DesiredStateRevisionStore,
	deployments DeploymentStore,
	bindings DeploymentBindingStore,
	routes PublicRouteStore,
	gatewayIntents GatewayConfigIntentStore,
	releases ReleaseHistoryStore,
) *GatewayConfigService {
	return &GatewayConfigService{
		revisions:      revisions,
		deployments:    deployments,
		bindings:       bindings,
		routes:         routes,
		gatewayIntents: gatewayIntents,
		releases:       releases,
	}
}

func (s *GatewayConfigService) AllocateMagicDomain(projectID, serviceName, rawPublicIP, provider string) (string, error) {
	publicIP := strings.TrimSpace(rawPublicIP)
	if publicIP == "" {
		return "", ErrInvalidMagicDomainIP
	}

	if net.ParseIP(publicIP) == nil {
		return "", fmt.Errorf("%w: %q is not a valid IP", ErrInvalidMagicDomainIP, publicIP)
	}

	if isPrivateIP(publicIP) {
		return "", fmt.Errorf("%w: %q is a private IP, magic domains require public IP", ErrInvalidMagicDomainIP, publicIP)
	}

	prov := strings.TrimSpace(provider)
	if prov == "" {
		prov = MagicDomainProviderSSLIP
	}
	if prov != MagicDomainProviderSSLIP && prov != MagicDomainProviderNipIO {
		prov = MagicDomainProviderSSLIP
	}

	domain := fmt.Sprintf("%s.%s.%s", serviceName, strings.ReplaceAll(publicIP, ".", "-"), prov)
	return domain, nil
}

func (s *GatewayConfigService) GenerateGatewayConfig(ctx context.Context, projectID, deploymentID, revisionID string) (*GatewayConfigPayload, error) {
	revision, err := s.revisions.GetByIDForProject(projectID, revisionID)
	if err != nil {
		return nil, err
	}
	if revision == nil {
		return nil, ErrRevisionNotFound
	}

	compiled, err := parseCompiledRevision(revision.CompiledRevisionJSON)
	if err != nil {
		return nil, fmt.Errorf("parse compiled revision: %w", err)
	}

	binding, err := s.bindings.GetByIDForProject(projectID, compiled.DeploymentBindingID)
	if err != nil {
		return nil, fmt.Errorf("resolve binding: %w", err)
	}
	if binding == nil {
		return nil, fmt.Errorf("deployment binding %q not found", compiled.DeploymentBindingID)
	}

	routes, err := s.routes.ListByDeployment(projectID, deploymentID)
	if err != nil {
		return nil, err
	}

	routeConfigs := make([]RouteConfig, 0, len(routes))
	for _, r := range routes {
		if r.Status != PublicRouteStatusActive {
			continue
		}
		routeConfigs = append(routeConfigs, RouteConfig{
			Domain:       r.Domain,
			ServiceName:  r.ServiceName,
			PathPrefix:   r.PathPrefix,
			UpstreamPort: r.UpstreamPort,
			HTTPS:        r.HTTPS,
		})
	}

	for _, svc := range compiled.Services {
		if svc.Public && !routeExists(routeConfigs, svc.Name) {
			domain := fmt.Sprintf("%s.%s.sslip.io", svc.Name, strings.ReplaceAll(publicIPFromPlacement(compiled.PlacementAssignments, svc.Name), ".", "-"))
			routeConfigs = append(routeConfigs, RouteConfig{
				Domain:       domain,
				ServiceName:  svc.Name,
				PathPrefix:   "/",
				UpstreamPort: extractPortFromHealthcheck(svc.Healthcheck),
				HTTPS:        true,
			})
		}
	}

	payload := &GatewayConfigPayload{
		ProjectID:           projectID,
		DeploymentID:        deploymentID,
		RevisionID:          revisionID,
		RuntimeMode:         binding.RuntimeMode,
		TargetKind:          binding.TargetKind,
		TargetID:            binding.TargetID,
		Routes:              routeConfigs,
		CompatibilityPolicy: compiled.CompatibilityPolicy,
		ScaleToZeroPolicy:   compiled.ScaleToZeroPolicy,
	}

	return payload, nil
}

func (s *GatewayConfigService) DispatchGatewayConfig(ctx context.Context, projectID, deploymentID, revisionID string) (*GatewayConfigDispatchResult, error) {
	config, err := s.GenerateGatewayConfig(ctx, projectID, deploymentID, revisionID)
	if err != nil {
		return nil, err
	}

	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	intent := &models.GatewayConfigIntent{
		ID:           utils.NewPrefixedID("gwi"),
		ProjectID:    projectID,
		DeploymentID: deploymentID,
		RevisionID:   revisionID,
		TargetKind:   config.TargetKind,
		TargetID:     config.TargetID,
		ConfigJSON:   string(configJSON),
		Status:       GatewayConfigStatusPending,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := s.gatewayIntents.Create(intent); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	if err := s.gatewayIntents.UpdateStatus(intent.ID, GatewayConfigStatusDispatched, now); err != nil {
		return nil, err
	}

	return &GatewayConfigDispatchResult{
		IntentID:     intent.ID,
		ProjectID:    projectID,
		Status:       GatewayConfigStatusDispatched,
		DispatchedAt: now,
	}, nil
}

func (s *GatewayConfigService) RecordRelease(projectID, deploymentID, revisionID, commitSHA, triggerKind, runtimeMode, status string) (*ReleaseRecord, error) {
	summaryJSON, _ := json.Marshal(map[string]any{
		"commit_sha":   commitSHA,
		"trigger_kind": triggerKind,
		"runtime_mode": runtimeMode,
	})

	record := &models.ReleaseHistory{
		ID:           utils.NewPrefixedID("rel"),
		ProjectID:    projectID,
		DeploymentID: deploymentID,
		RevisionID:   revisionID,
		CommitSHA:    commitSHA,
		TriggerKind:  triggerKind,
		Status:       status,
		RuntimeMode:  runtimeMode,
		SummaryJSON:  string(summaryJSON),
		CreatedAt:    time.Now().UTC(),
	}

	if status == ReleaseStatusDeployed || status == ReleaseStatusPromoted {
		now := time.Now().UTC()
		record.DeployedAt = &now
	}

	if err := s.releases.Create(record); err != nil {
		return nil, err
	}

	r := toReleaseRecord(*record)
	return &r, nil
}

func (s *GatewayConfigService) ListReleaseHistory(projectID string, limit int) ([]ReleaseRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	items, err := s.releases.ListByProject(projectID, limit)
	if err != nil {
		return nil, err
	}

	out := make([]ReleaseRecord, len(items))
	for i, item := range items {
		out[i] = toReleaseRecord(item)
	}
	return out, nil
}

func (s *GatewayConfigService) GetReleaseDetail(projectID, recordID string) (*ReleaseRecord, error) {
	record, err := s.releases.GetByIDForProject(projectID, recordID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, ErrReleaseNotFound
	}

	r := toReleaseRecord(*record)
	return &r, nil
}

func (s *GatewayConfigService) CreatePublicRoute(projectID, deploymentID, serviceName, domain, domainKind, pathPrefix string, upstreamPort int, https bool) (*PublicRouteRecord, error) {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return nil, ErrInvalidDomain
	}

	dk := strings.TrimSpace(domainKind)
	if dk == "" {
		dk = DomainKindMagic
	}

	route := &models.PublicRoute{
		ID:           utils.NewPrefixedID("prt"),
		ProjectID:    projectID,
		DeploymentID: deploymentID,
		ServiceName:  strings.TrimSpace(serviceName),
		Domain:       domain,
		DomainKind:   dk,
		PathPrefix:   normalizePathPrefix(pathPrefix),
		UpstreamPort: upstreamPort,
		HTTPS:        https,
		Status:       PublicRouteStatusActive,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	if err := s.routes.Create(route); err != nil {
		return nil, err
	}

	return toPublicRouteRecord(*route), nil
}

type GatewayConfigPayload struct {
	ProjectID           string                         `json:"project_id"`
	DeploymentID        string                         `json:"deployment_id"`
	RevisionID          string                         `json:"revision_id"`
	RuntimeMode         string                         `json:"runtime_mode"`
	TargetKind          string                         `json:"target_kind"`
	TargetID            string                         `json:"target_id"`
	Routes              []RouteConfig                  `json:"routes"`
	CompatibilityPolicy LazyopsYAMLCompatibilityPolicy `json:"compatibility_policy"`
	ScaleToZeroPolicy   LazyopsYAMLScaleToZeroPolicy   `json:"scale_to_zero_policy"`
	WakeUpHold          WakeUpHoldConfig               `json:"wake_up_hold"`
}

type RouteConfig struct {
	Domain       string `json:"domain"`
	ServiceName  string `json:"service_name"`
	PathPrefix   string `json:"path_prefix"`
	UpstreamPort int    `json:"upstream_port"`
	HTTPS        bool   `json:"https"`
}

type WakeUpHoldConfig struct {
	Enabled      bool   `json:"enabled"`
	TimeoutMs    int    `json:"timeout_ms"`
	HoldResponse string `json:"hold_response"`
}

type GatewayConfigDispatchResult struct {
	IntentID     string    `json:"intent_id"`
	ProjectID    string    `json:"project_id"`
	Status       string    `json:"status"`
	DispatchedAt time.Time `json:"dispatched_at"`
}

type ReleaseRecord struct {
	ID           string         `json:"id"`
	ProjectID    string         `json:"project_id"`
	DeploymentID string         `json:"deployment_id"`
	RevisionID   string         `json:"revision_id"`
	CommitSHA    string         `json:"commit_sha"`
	TriggerKind  string         `json:"trigger_kind"`
	Status       string         `json:"status"`
	RuntimeMode  string         `json:"runtime_mode"`
	Summary      map[string]any `json:"summary"`
	DeployedAt   *time.Time     `json:"deployed_at"`
	CreatedAt    time.Time      `json:"created_at"`
}

type PublicRouteRecord struct {
	ID           string    `json:"id"`
	ProjectID    string    `json:"project_id"`
	DeploymentID string    `json:"deployment_id"`
	ServiceName  string    `json:"service_name"`
	Domain       string    `json:"domain"`
	DomainKind   string    `json:"domain_kind"`
	PathPrefix   string    `json:"path_prefix"`
	UpstreamPort int       `json:"upstream_port"`
	HTTPS        bool      `json:"https"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

func toReleaseRecord(item models.ReleaseHistory) ReleaseRecord {
	var summary map[string]any
	if item.SummaryJSON != "" {
		_ = json.Unmarshal([]byte(item.SummaryJSON), &summary)
	}
	return ReleaseRecord{
		ID:           item.ID,
		ProjectID:    item.ProjectID,
		DeploymentID: item.DeploymentID,
		RevisionID:   item.RevisionID,
		CommitSHA:    item.CommitSHA,
		TriggerKind:  item.TriggerKind,
		Status:       item.Status,
		RuntimeMode:  item.RuntimeMode,
		Summary:      summary,
		DeployedAt:   item.DeployedAt,
		CreatedAt:    item.CreatedAt,
	}
}

func toPublicRouteRecord(item models.PublicRoute) *PublicRouteRecord {
	return &PublicRouteRecord{
		ID:           item.ID,
		ProjectID:    item.ProjectID,
		DeploymentID: item.DeploymentID,
		ServiceName:  item.ServiceName,
		Domain:       item.Domain,
		DomainKind:   item.DomainKind,
		PathPrefix:   item.PathPrefix,
		UpstreamPort: item.UpstreamPort,
		HTTPS:        item.HTTPS,
		Status:       item.Status,
		CreatedAt:    item.CreatedAt,
	}
}

func isPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()
}

func normalizePathPrefix(raw string) string {
	prefix := strings.TrimSpace(raw)
	if prefix == "" {
		return "/"
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	return prefix
}

func routeExists(routes []RouteConfig, serviceName string) bool {
	for _, r := range routes {
		if r.ServiceName == serviceName {
			return true
		}
	}
	return false
}

func extractPortFromHealthcheck(hc map[string]any) int {
	if hc == nil {
		return 8080
	}
	if port, ok := hc["port"].(float64); ok {
		return int(port)
	}
	if port, ok := hc["port"].(int); ok {
		return port
	}
	return 8080
}

func publicIPFromPlacement(assignments []PlacementAssignmentRecord, serviceName string) string {
	for _, a := range assignments {
		if a.ServiceName == serviceName {
			return "127.0.0.1"
		}
	}
	return "127.0.0.1"
}
