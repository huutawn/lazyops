package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"lazyops-server/internal/config"
	"lazyops-server/internal/models"
	"lazyops-server/pkg/utils"
)

var (
	ErrProjectDomainNotFound     = errors.New("project domain not found")
	ErrProjectDomainLabelInvalid = errors.New("project domain label is invalid")
	ErrProjectDomainLabelTaken   = errors.New("project domain label already in use")
)

var projectDomainLabelPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

type ProjectDomainService struct {
	projects  ProjectStore
	domains   ProjectDomainStore
	services  ProjectServiceStore
	bindings  DeploymentBindingStore
	instances InstanceStore
	clusters  ClusterStore
	cfg       config.PublicDomainConfig
	dnsClient ProjectDomainDNSClient
}

func NewProjectDomainService(
	projects ProjectStore,
	domains ProjectDomainStore,
	services ProjectServiceStore,
	bindings DeploymentBindingStore,
	instances InstanceStore,
	clusters ClusterStore,
	cfg config.PublicDomainConfig,
) *ProjectDomainService {
	return (&ProjectDomainService{
		projects:  projects,
		domains:   domains,
		services:  services,
		bindings:  bindings,
		instances: instances,
		clusters:  clusters,
		cfg:       cfg,
	}).WithDNSClient(NewProjectDomainDNSClient(cfg))
}

func (s *ProjectDomainService) WithDNSClient(client ProjectDomainDNSClient) *ProjectDomainService {
	if s == nil {
		return s
	}
	if client == nil {
		client = &NoopProjectDomainDNSClient{}
	}
	s.dnsClient = client
	return s
}

func (s *ProjectDomainService) Get(requesterUserID, requesterRole, projectID string) (*ProjectDomainRecord, error) {
	project, err := resolveProjectForAccess(s.projects, requesterUserID, requesterRole, projectID)
	if err != nil {
		return nil, err
	}
	return s.GetPrimaryManagedByProjectID(project.ID)
}

func (s *ProjectDomainService) GetPrimaryManagedByProjectID(projectID string) (*ProjectDomainRecord, error) {
	if s == nil || s.domains == nil || strings.TrimSpace(projectID) == "" {
		return nil, ErrInvalidInput
	}
	item, err := s.domains.GetByProjectIDAndKind(projectID, ProjectDomainKindManaged)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrProjectDomainNotFound
	}
	record := toProjectDomainRecord(*item)
	return &record, nil
}

func (s *ProjectDomainService) Allocate(cmd AllocateProjectDomainCommand) (*ProjectDomainRecord, error) {
	project, err := resolveProjectForAccess(s.projects, cmd.RequesterUserID, cmd.RequesterRole, cmd.ProjectID)
	if err != nil {
		return nil, err
	}
	if !s.projectHasPublicServices(project.ID) {
		return nil, ErrInvalidInput
	}

	targetKind, targetID, publicIP, err := s.resolvePrimaryTarget(project.ID)
	if err != nil {
		return nil, err
	}
	if cmd.Regenerate {
		domain, err := s.ensureManagedDomain(project.ID, project.Slug, targetKind, targetID, publicIP, true)
		if err != nil {
			return nil, err
		}
		record := toProjectDomainRecord(*domain)
		return &record, nil
	}

	domain, err := s.ensureManagedDomain(project.ID, project.Slug, targetKind, targetID, publicIP, false)
	if err != nil {
		return nil, err
	}
	record := toProjectDomainRecord(*domain)
	return &record, nil
}

func (s *ProjectDomainService) Rename(cmd RenameProjectDomainCommand) (*ProjectDomainRecord, error) {
	project, err := resolveProjectForAccess(s.projects, cmd.RequesterUserID, cmd.RequesterRole, cmd.ProjectID)
	if err != nil {
		return nil, err
	}

	existing, err := s.domains.GetByProjectIDAndKind(project.ID, ProjectDomainKindManaged)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrProjectDomainNotFound
	}

	label, err := normalizeProjectDomainLabel(cmd.Label)
	if err != nil {
		return nil, err
	}
	hostname := s.hostnameForLabel(label)
	if err := s.ensureHostnameAvailable(project.ID, hostname); err != nil {
		return nil, err
	}

	targetKind, targetID, publicIP, err := s.resolvePrimaryTarget(project.ID)
	if err != nil {
		return nil, err
	}

	existing.Label = label
	existing.Hostname = hostname
	existing.TargetKind = strings.TrimSpace(targetKind)
	existing.TargetID = strings.TrimSpace(targetID)
	existing.Status = ProjectDomainStatusPending
	existing.StatusReason = ""
	if err := s.syncManagedDomain(context.Background(), existing, publicIP); err != nil {
		return nil, err
	}
	if err := s.domains.Save(existing); err != nil {
		return nil, err
	}
	record := toProjectDomainRecord(*existing)
	return &record, nil
}

func (s *ProjectDomainService) EnsureManagedDomain(projectID, projectSlug, targetKind, targetID, publicIP string) (*ProjectDomainRecord, error) {
	item, err := s.ensureManagedDomain(projectID, projectSlug, targetKind, targetID, publicIP, false)
	if err != nil {
		return nil, err
	}
	record := toProjectDomainRecord(*item)
	return &record, nil
}

func (s *ProjectDomainService) ensureManagedDomain(projectID, projectSlug, targetKind, targetID, publicIP string, regenerate bool) (*models.ProjectDomain, error) {
	if s == nil || s.domains == nil || strings.TrimSpace(projectID) == "" {
		return nil, ErrInvalidInput
	}

	existing, err := s.domains.GetByProjectIDAndKind(projectID, ProjectDomainKindManaged)
	if err != nil {
		return nil, err
	}

	if existing == nil {
		label, hostname, err := s.allocateUniqueHostname(projectSlug)
		if err != nil {
			return nil, err
		}
		existing = &models.ProjectDomain{
			ID:        utils.NewPrefixedID("dom"),
			ProjectID: projectID,
			Label:     label,
			Hostname:  hostname,
			Kind:      ProjectDomainKindManaged,
			Status:    ProjectDomainStatusPending,
		}
	} else if regenerate {
		label, hostname, err := s.allocateUniqueHostname(projectSlug)
		if err != nil {
			return nil, err
		}
		existing.Label = label
		existing.Hostname = hostname
	}

	existing.TargetKind = strings.TrimSpace(targetKind)
	existing.TargetID = strings.TrimSpace(targetID)
	if err := s.syncManagedDomain(context.Background(), existing, publicIP); err != nil {
		return nil, err
	}

	if existing.CreatedAt.IsZero() {
		now := time.Now().UTC()
		existing.CreatedAt = now
		existing.UpdatedAt = now
		if err := s.domains.Create(existing); err != nil {
			return nil, err
		}
		return existing, nil
	}
	existing.UpdatedAt = time.Now().UTC()
	if err := s.domains.Save(existing); err != nil {
		return nil, err
	}
	return existing, nil
}

func (s *ProjectDomainService) syncManagedDomain(ctx context.Context, item *models.ProjectDomain, publicIP string) error {
	if item == nil {
		return ErrInvalidInput
	}
	publicIP = strings.TrimSpace(publicIP)
	if publicIP == "" || net.ParseIP(publicIP) == nil {
		item.Status = ProjectDomainStatusPending
		item.StatusReason = "Đang chờ public IP để đồng bộ DNS."
		item.LastSyncedIP = ""
		return nil
	}
	if item.Status == ProjectDomainStatusActive &&
		strings.TrimSpace(item.LastSyncedIP) == publicIP &&
		strings.TrimSpace(item.CloudflareRecordID) != "" {
		item.StatusReason = ""
		return nil
	}
	result, err := s.dnsClient.UpsertARecord(ctx, ProjectDomainDNSUpsertRequest{
		Hostname:         item.Hostname,
		IPv4:             publicIP,
		Proxied:          s.cfg.CloudflareProxied,
		ExistingRecordID: item.CloudflareRecordID,
	})
	if err != nil {
		item.Status = ProjectDomainStatusError
		item.StatusReason = fmt.Sprintf("Không thể đồng bộ DNS Cloudflare: %v", err)
		return nil
	}
	item.Status = ProjectDomainStatusActive
	item.StatusReason = ""
	item.CloudflareRecordID = firstNonEmptyCompiledValue(result.RecordID, item.CloudflareRecordID)
	item.LastSyncedIP = publicIP
	return nil
}

func (s *ProjectDomainService) resolvePrimaryTarget(projectID string) (string, string, string, error) {
	if s == nil || s.bindings == nil {
		return "", "", "", nil
	}
	binding, err := s.bindings.GetByTargetRefForProject(projectID, "auto-primary")
	if err != nil {
		return "", "", "", err
	}
	if binding == nil {
		items, err := s.bindings.ListByProject(projectID)
		if err != nil {
			return "", "", "", err
		}
		if len(items) == 0 {
			return "", "", "", nil
		}
		binding = &items[0]
	}
	return strings.TrimSpace(binding.TargetKind), strings.TrimSpace(binding.TargetID), s.resolveTargetPublicIP(binding.TargetKind, binding.TargetID), nil
}

func (s *ProjectDomainService) resolveTargetPublicIP(targetKind, targetID string) string {
	switch strings.TrimSpace(targetKind) {
	case "instance":
		if s.instances == nil || strings.TrimSpace(targetID) == "" {
			return ""
		}
		instance, err := s.instances.GetByID(strings.TrimSpace(targetID))
		if err != nil || instance == nil || instance.PublicIP == nil {
			return ""
		}
		return strings.TrimSpace(*instance.PublicIP)
	case "cluster":
		if s.clusters == nil || strings.TrimSpace(targetID) == "" {
			return ""
		}
		cluster, err := s.clusters.GetByID(strings.TrimSpace(targetID))
		if err != nil || cluster == nil || cluster.PublicIP == nil {
			return ""
		}
		return strings.TrimSpace(*cluster.PublicIP)
	default:
		return ""
	}
}

func (s *ProjectDomainService) projectHasPublicServices(projectID string) bool {
	if s == nil || s.services == nil || strings.TrimSpace(projectID) == "" {
		return false
	}
	items, err := s.services.ListByProject(projectID)
	if err != nil {
		return false
	}
	for _, item := range items {
		if item.Public {
			return true
		}
	}
	return false
}

func (s *ProjectDomainService) allocateUniqueHostname(projectSlug string) (string, string, error) {
	base := sanitizeProjectDomainLabel(projectSlug)
	if base == "" {
		base = "app"
	}
	for attempt := 0; attempt < 8; attempt++ {
		suffix, err := randomProjectDomainSuffix()
		if err != nil {
			return "", "", err
		}
		label := trimProjectDomainLabel(fmt.Sprintf("%s-%s", base, suffix))
		hostname := s.hostnameForLabel(label)
		existing, err := s.domains.GetByHostname(hostname)
		if err != nil {
			return "", "", err
		}
		if existing == nil {
			return label, hostname, nil
		}
	}
	return "", "", fmt.Errorf("could not allocate a unique managed domain label")
}

func (s *ProjectDomainService) hostnameForLabel(label string) string {
	baseDomain := strings.Trim(strings.ToLower(strings.TrimSpace(s.cfg.BaseDomain)), ".")
	if baseDomain == "" {
		baseDomain = "lazyops.cloud"
	}
	return label + "." + baseDomain
}

func (s *ProjectDomainService) ensureHostnameAvailable(projectID, hostname string) error {
	existing, err := s.domains.GetByHostname(hostname)
	if err != nil {
		return err
	}
	if existing != nil && strings.TrimSpace(existing.ProjectID) != strings.TrimSpace(projectID) {
		return ErrProjectDomainLabelTaken
	}
	return nil
}

func toProjectDomainRecord(item models.ProjectDomain) ProjectDomainRecord {
	return ProjectDomainRecord{
		ID:                 item.ID,
		ProjectID:          item.ProjectID,
		Hostname:           item.Hostname,
		Label:              item.Label,
		Kind:               item.Kind,
		Status:             item.Status,
		StatusReason:       item.StatusReason,
		CloudflareRecordID: item.CloudflareRecordID,
		TargetKind:         item.TargetKind,
		TargetID:           item.TargetID,
		LastSyncedIP:       item.LastSyncedIP,
		PublicURL:          managedProjectDomainURL(item.Hostname),
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
	}
}

func managedProjectDomainURL(hostname string) string {
	host := strings.TrimSpace(hostname)
	if host == "" {
		return ""
	}
	return "https://" + host
}

func normalizeProjectDomainLabel(raw string) (string, error) {
	label := trimProjectDomainLabel(strings.ToLower(strings.TrimSpace(raw)))
	if !projectDomainLabelPattern.MatchString(label) {
		return "", ErrProjectDomainLabelInvalid
	}
	return label, nil
}

func sanitizeProjectDomainLabel(raw string) string {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, r := range trimmed {
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
	return trimProjectDomainLabel(b.String())
}

func trimProjectDomainLabel(label string) string {
	label = strings.Trim(strings.ToLower(strings.TrimSpace(label)), "-")
	if len(label) <= 63 {
		return label
	}
	return strings.Trim(label[:63], "-")
}

func randomProjectDomainSuffix() (string, error) {
	buf := make([]byte, 3)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate project domain suffix: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
