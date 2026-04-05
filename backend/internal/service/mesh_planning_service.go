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
	TunnelSessionStatusActive  = "active"
	TunnelSessionStatusClosed  = "closed"
	TunnelSessionStatusExpired = "expired"

	TunnelSessionTypeDB  = "db"
	TunnelSessionTypeTCP = "tcp"

	TopologyStateOnline   = "online"
	TopologyStateOffline  = "offline"
	TopologyStateDegraded = "degraded"

	AllowedMeshProtocolsTCP  = "tcp"
	AllowedMeshProtocolsHTTP = "http"
	AllowedMeshProtocolsGRPC = "grpc"
	AllowedMeshProtocolsTLS  = "tls"
)

var (
	ErrUnsupportedProtocol       = errors.New("unsupported protocol for mesh dependency binding")
	ErrTargetOffline             = errors.New("target instance is offline")
	ErrTunnelSessionExpired      = errors.New("tunnel session expired")
	ErrTunnelSessionPortConflict = errors.New("local port already in use by active tunnel session")
	ErrTunnelSessionCloseFailed  = errors.New("failed to close expired tunnel session")
)

var allowedMeshProtocols = map[string]struct{}{
	AllowedMeshProtocolsTCP:  {},
	AllowedMeshProtocolsHTTP: {},
	AllowedMeshProtocolsGRPC: {},
	AllowedMeshProtocolsTLS:  {},
}

type MeshPlanningService struct {
	instances InstanceStore
	bindings  DeploymentBindingStore
	revisions DesiredStateRevisionStore
	tunnels   TunnelSessionStore
	topology  TopologyStateStore
}

type TunnelSessionStore interface {
	Create(session *models.TunnelSession) error
	GetByID(sessionID string) (*models.TunnelSession, error)
	ListByTarget(targetKind, targetID string) ([]models.TunnelSession, error)
	CloseSession(sessionID string, at time.Time) error
	CleanupExpired(before time.Time) error
}

type TopologyStateStore interface {
	Upsert(state *models.TopologyState) error
	GetByInstance(instanceID string) (*models.TopologyState, error)
	ListByProject(projectID string) ([]models.TopologyState, error)
	ListActiveByMesh(meshID string) ([]models.TopologyState, error)
}

func NewMeshPlanningService(
	instances InstanceStore,
	bindings DeploymentBindingStore,
	revisions DesiredStateRevisionStore,
	tunnels TunnelSessionStore,
	topology TopologyStateStore,
) *MeshPlanningService {
	return &MeshPlanningService{
		instances: instances,
		bindings:  bindings,
		revisions: revisions,
		tunnels:   tunnels,
		topology:  topology,
	}
}

func (s *MeshPlanningService) ResolveDependencyBinding(ctx context.Context, projectID string, serviceName string, binding LazyopsYAMLDependencyBinding) (*DependencyResolutionResult, error) {
	protocol := strings.ToLower(strings.TrimSpace(binding.Protocol))
	if _, ok := allowedMeshProtocols[protocol]; !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedProtocol, binding.Protocol)
	}

	targetInstance, err := s.findInstanceForService(ctx, projectID, binding.TargetService)
	if err != nil {
		return nil, err
	}

	topoState, err := s.topology.GetByInstance(targetInstance.ID)
	if err != nil {
		return nil, err
	}

	if topoState == nil || topoState.State == TopologyStateOffline {
		return nil, fmt.Errorf("%w: instance %q for service %q", ErrTargetOffline, targetInstance.ID, binding.TargetService)
	}

	targetIP := resolveInstanceIP(*targetInstance)
	if targetIP == "" {
		return nil, fmt.Errorf("no routable IP for instance %q", targetInstance.ID)
	}

	serviceHealthcheckPort := s.resolveServiceHealthcheckPort(ctx, projectID, binding.TargetService)
	targetPort := extractPortFromDependencyBinding(binding, serviceHealthcheckPort)
	if targetPort == 0 {
		return nil, fmt.Errorf("cannot resolve port for service %q: no local_endpoint configured and no healthcheck port available", binding.TargetService)
	}

	resolution := &DependencyResolutionResult{
		ServiceName:    serviceName,
		TargetService:  binding.TargetService,
		Alias:          binding.Alias,
		Protocol:       protocol,
		LocalEndpoint:  binding.LocalEndpoint,
		TargetEndpoint: fmt.Sprintf("%s:%d", targetIP, targetPort),
		TargetKind:     "instance",
		TargetID:       targetInstance.ID,
		PrivatePath:    s.buildPrivatePath(targetIP, targetPort, protocol),
		EnvInjection:   s.buildEnvInjection(serviceName, binding.Alias, targetIP, targetPort, protocol),
	}

	return resolution, nil
}

func (s *MeshPlanningService) ResolveAllDependencies(ctx context.Context, projectID string, deps []LazyopsYAMLDependencyBinding) ([]DependencyResolutionResult, error) {
	results := make([]DependencyResolutionResult, 0, len(deps))

	for _, dep := range deps {
		result, err := s.ResolveDependencyBinding(ctx, projectID, dep.Service, dep)
		if err != nil {
			return nil, fmt.Errorf("resolve dependency %s -> %s: %w", dep.Service, dep.TargetService, err)
		}
		results = append(results, *result)
	}

	return results, nil
}

func (s *MeshPlanningService) CreateTunnelSession(ctx context.Context, projectID, targetKind, targetID, sessionType string, localPort, remotePort int, ttl time.Duration) (*TunnelSessionRecord, error) {
	instance, err := s.findTargetInstance(ctx, projectID, targetKind, targetID)
	if err != nil {
		return nil, err
	}

	topoState, err := s.topology.GetByInstance(instance.ID)
	if err != nil {
		return nil, err
	}
	if topoState == nil || topoState.State == TopologyStateOffline {
		return nil, fmt.Errorf("%w: cannot create tunnel to offline target", ErrTargetOffline)
	}

	existingSessions, err := s.tunnels.ListByTarget(targetKind, targetID)
	if err != nil {
		return nil, fmt.Errorf("check existing tunnel sessions: %w", err)
	}
	now := time.Now().UTC()
	for _, existing := range existingSessions {
		if existing.Status == TunnelSessionStatusActive && existing.LocalPort == localPort {
			if existing.ExpiresAt.After(now) {
				return nil, fmt.Errorf("local port %d already in use by active tunnel session %q", localPort, existing.ID)
			}
			if err := s.tunnels.CloseSession(existing.ID, now); err != nil {
				return nil, fmt.Errorf("close expired tunnel session %q before creating new one: %w", existing.ID, err)
			}
		}
	}

	expiresAt := now.Add(ttl)

	session := &models.TunnelSession{
		ID:          utils.NewPrefixedID("tun"),
		ProjectID:   projectID,
		TargetKind:  targetKind,
		TargetID:    targetID,
		InstanceID:  instance.ID,
		SessionType: sessionType,
		LocalPort:   localPort,
		RemotePort:  remotePort,
		Status:      TunnelSessionStatusActive,
		Token:       utils.NewPrefixedID("tok"),
		ExpiresAt:   expiresAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.tunnels.Create(session); err != nil {
		return nil, err
	}

	return toTunnelSessionRecord(*session), nil
}

func (s *MeshPlanningService) CloseTunnelSession(ctx context.Context, sessionID string) (*TunnelSessionRecord, error) {
	session, err := s.tunnels.GetByID(sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrTunnelSessionNotFound
	}

	now := time.Now().UTC()
	if err := s.tunnels.CloseSession(sessionID, now); err != nil {
		return nil, err
	}

	session.Status = TunnelSessionStatusClosed
	session.UpdatedAt = now

	return toTunnelSessionRecord(*session), nil
}

func (s *MeshPlanningService) IngestTopologyState(ctx context.Context, instanceID, meshID, state string, metadata map[string]any) (*TopologyStateRecord, error) {
	now := time.Now().UTC()

	topoState := &models.TopologyState{
		ID:           utils.NewPrefixedID("topo"),
		InstanceID:   instanceID,
		MeshID:       meshID,
		State:        normalizeTopologyState(state),
		MetadataJSON: marshalOrEmpty(metadata),
		LastSeenAt:   now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.topology.Upsert(topoState); err != nil {
		return nil, err
	}

	r := toTopologyStateRecord(*topoState)
	return &r, nil
}

func (s *MeshPlanningService) ListMeshTopology(projectID string) ([]TopologyStateRecord, error) {
	states, err := s.topology.ListByProject(projectID)
	if err != nil {
		return nil, err
	}

	out := make([]TopologyStateRecord, len(states))
	for i, state := range states {
		out[i] = toTopologyStateRecord(state)
	}
	return out, nil
}

func (s *MeshPlanningService) buildPrivatePath(targetIP string, targetPort int, protocol string) PrivatePathConfig {
	return PrivatePathConfig{
		Via:        "mesh",
		TargetIP:   targetIP,
		TargetPort: targetPort,
		Protocol:   protocol,
		Encrypted:  true,
	}
}

func (s *MeshPlanningService) buildEnvInjection(serviceName, alias, targetIP string, targetPort int, protocol string) map[string]string {
	hostVar := strings.ToUpper(strings.ReplaceAll(alias, "-", "_")) + "_HOST"
	portVar := strings.ToUpper(strings.ReplaceAll(alias, "-", "_")) + "_PORT"
	urlVar := strings.ToUpper(strings.ReplaceAll(alias, "-", "_")) + "_URL"

	scheme := "http"
	if protocol == AllowedMeshProtocolsTLS || protocol == AllowedMeshProtocolsGRPC {
		scheme = "https"
	}

	return map[string]string{
		hostVar: targetIP,
		portVar: fmt.Sprintf("%d", targetPort),
		urlVar:  fmt.Sprintf("%s://%s:%d", scheme, targetIP, targetPort),
	}
}

func (s *MeshPlanningService) findInstanceForService(ctx context.Context, projectID, serviceName string) (*models.Instance, error) {
	bindings, err := s.bindings.ListByProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("resolve project bindings for service %q: %w", serviceName, err)
	}
	if len(bindings) == 0 {
		return nil, fmt.Errorf("no deployment binding found for project %q", projectID)
	}

	var fallbackInstance *models.Instance
	for _, binding := range bindings {
		if binding.TargetKind != "instance" {
			continue
		}
		instance, err := s.instances.GetByID(binding.TargetID)
		if err != nil {
			continue
		}
		if instance == nil {
			continue
		}
		if strings.EqualFold(instance.Status, "offline") {
			continue
		}
		if fallbackInstance == nil {
			fallbackInstance = instance
		}

		if strings.EqualFold(binding.Name, serviceName) {
			return instance, nil
		}

		revisions, err := s.revisions.ListByProject(projectID)
		if err != nil {
			continue
		}
		for _, rev := range revisions {
			if rev.DeploymentBindingID != binding.ID {
				continue
			}
			compiled, err := parseCompiledRevision(rev.CompiledRevisionJSON)
			if err != nil {
				continue
			}
			for _, assignment := range compiled.PlacementAssignments {
				if assignment.ServiceName == serviceName && assignment.TargetID == binding.TargetID {
					return instance, nil
				}
			}
		}
	}

	if fallbackInstance != nil {
		return fallbackInstance, nil
	}

	return nil, fmt.Errorf("no online instance found for service %q in project %q", serviceName, projectID)
}

func (s *MeshPlanningService) findTargetInstance(ctx context.Context, projectID, targetKind, targetID string) (*models.Instance, error) {
	if targetKind != "instance" {
		return nil, fmt.Errorf("tunnel sessions only support instance targets, got %q", targetKind)
	}

	instance, err := s.instances.GetByID(targetID)
	if err != nil {
		return nil, err
	}
	if instance == nil {
		return nil, ErrTargetNotFound
	}

	return instance, nil
}

func resolveInstanceIP(instance models.Instance) string {
	if instance.PrivateIP != nil && *instance.PrivateIP != "" {
		return *instance.PrivateIP
	}
	if instance.PublicIP != nil && *instance.PublicIP != "" {
		return *instance.PublicIP
	}
	return ""
}

func extractPortFromDependencyBinding(binding LazyopsYAMLDependencyBinding, serviceHealthcheckPort int) int {
	if binding.LocalEndpoint != "" {
		_, port, err := net.SplitHostPort(binding.LocalEndpoint)
		if err == nil {
			var p int
			fmt.Sscanf(port, "%d", &p)
			if p > 0 {
				return p
			}
		}
	}
	if serviceHealthcheckPort > 0 {
		return serviceHealthcheckPort
	}
	return 0
}

func (s *MeshPlanningService) resolveServiceHealthcheckPort(ctx context.Context, projectID, serviceName string) int {
	bindings, err := s.bindings.ListByProject(projectID)
	if err != nil {
		return 0
	}

	for _, binding := range bindings {
		if binding.TargetKind != "instance" {
			continue
		}
		revisions, err := s.revisions.ListByProject(projectID)
		if err != nil {
			continue
		}
		for _, rev := range revisions {
			if rev.DeploymentBindingID != binding.ID {
				continue
			}
			compiled, err := parseCompiledRevision(rev.CompiledRevisionJSON)
			if err != nil {
				continue
			}
			for _, svc := range compiled.Services {
				if svc.Name == serviceName && svc.Healthcheck != nil {
					if portVal, ok := svc.Healthcheck["port"]; ok {
						switch p := portVal.(type) {
						case float64:
							if int(p) > 0 {
								return int(p)
							}
						case int:
							if p > 0 {
								return p
							}
						}
					}
				}
			}
		}
	}
	return 0
}

func normalizeTopologyState(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case TopologyStateOnline:
		return TopologyStateOnline
	case TopologyStateOffline:
		return TopologyStateOffline
	case TopologyStateDegraded:
		return TopologyStateDegraded
	default:
		return TopologyStateOffline
	}
}

func marshalOrEmpty(v any) string {
	if v == nil {
		return "{}"
	}
	data, _ := json.Marshal(v)
	if len(data) == 0 {
		return "{}"
	}
	return string(data)
}

type DependencyResolutionResult struct {
	ServiceName    string            `json:"service_name"`
	TargetService  string            `json:"target_service"`
	Alias          string            `json:"alias"`
	Protocol       string            `json:"protocol"`
	LocalEndpoint  string            `json:"local_endpoint"`
	TargetEndpoint string            `json:"target_endpoint"`
	TargetKind     string            `json:"target_kind"`
	TargetID       string            `json:"target_id"`
	PrivatePath    PrivatePathConfig `json:"private_path"`
	EnvInjection   map[string]string `json:"env_injection"`
}

type PrivatePathConfig struct {
	Via        string `json:"via"`
	TargetIP   string `json:"target_ip"`
	TargetPort int    `json:"target_port"`
	Protocol   string `json:"protocol"`
	Encrypted  bool   `json:"encrypted"`
}

type TunnelSessionRecord struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	TargetKind  string    `json:"target_kind"`
	TargetID    string    `json:"target_id"`
	InstanceID  string    `json:"instance_id"`
	SessionType string    `json:"session_type"`
	LocalPort   int       `json:"local_port"`
	RemotePort  int       `json:"remote_port"`
	Status      string    `json:"status"`
	Token       string    `json:"token"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
}

type TopologyStateRecord struct {
	ID         string         `json:"id"`
	InstanceID string         `json:"instance_id"`
	MeshID     string         `json:"mesh_id"`
	State      string         `json:"state"`
	Metadata   map[string]any `json:"metadata"`
	LastSeenAt time.Time      `json:"last_seen_at"`
	CreatedAt  time.Time      `json:"created_at"`
}

var ErrTunnelSessionNotFound = errors.New("tunnel session not found")

func toTunnelSessionRecord(item models.TunnelSession) *TunnelSessionRecord {
	return &TunnelSessionRecord{
		ID:          item.ID,
		ProjectID:   item.ProjectID,
		TargetKind:  item.TargetKind,
		TargetID:    item.TargetID,
		InstanceID:  item.InstanceID,
		SessionType: item.SessionType,
		LocalPort:   item.LocalPort,
		RemotePort:  item.RemotePort,
		Status:      item.Status,
		Token:       item.Token,
		ExpiresAt:   item.ExpiresAt,
		CreatedAt:   item.CreatedAt,
	}
}

func toTopologyStateRecord(item models.TopologyState) TopologyStateRecord {
	var metadata map[string]any
	if item.MetadataJSON != "" {
		_ = json.Unmarshal([]byte(item.MetadataJSON), &metadata)
	}
	return TopologyStateRecord{
		ID:         item.ID,
		InstanceID: item.InstanceID,
		MeshID:     item.MeshID,
		State:      item.State,
		Metadata:   metadata,
		LastSeenAt: item.LastSeenAt,
		CreatedAt:  item.CreatedAt,
	}
}
