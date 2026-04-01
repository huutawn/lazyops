package service

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"time"
	"unicode"

	"lazyops-server/internal/config"
	"lazyops-server/internal/models"
	"lazyops-server/pkg/utils"
)

var (
	ErrInstanceNameExists = errors.New("instance name already exists")
	ErrInvalidIP          = errors.New("invalid ip")
)

type InstanceService struct {
	instances  InstanceStore
	bootstrap  BootstrapTokenStore
	enrollment config.EnrollmentConfig
}

func NewInstanceService(
	instances InstanceStore,
	bootstrap BootstrapTokenStore,
	enrollment config.EnrollmentConfig,
) *InstanceService {
	return &InstanceService{
		instances:  instances,
		bootstrap:  bootstrap,
		enrollment: enrollment,
	}
}

func (s *InstanceService) Create(cmd CreateInstanceCommand) (*CreateInstanceResult, error) {
	userID := strings.TrimSpace(cmd.UserID)
	name := utils.NormalizeSpace(cmd.Name)
	if userID == "" || name == "" || len(name) > 255 {
		return nil, ErrInvalidInput
	}

	publicIP, privateIP, err := normalizeInstanceIPs(cmd.PublicIP, cmd.PrivateIP)
	if err != nil {
		return nil, err
	}

	labels, err := normalizeInstanceLabels(cmd.Labels)
	if err != nil {
		return nil, err
	}

	existing, err := s.instances.GetByNameForUser(userID, name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrInstanceNameExists
	}

	labelsJSON, err := json.Marshal(labels)
	if err != nil {
		return nil, err
	}
	runtimeCapabilitiesJSON, err := json.Marshal(map[string]any{})
	if err != nil {
		return nil, err
	}

	instance := &models.Instance{
		ID:                      utils.NewPrefixedID("inst"),
		UserID:                  userID,
		Name:                    name,
		PublicIP:                publicIP,
		PrivateIP:               privateIP,
		Status:                  "pending_enrollment",
		LabelsJSON:              string(labelsJSON),
		RuntimeCapabilitiesJSON: string(runtimeCapabilitiesJSON),
	}
	if err := s.instances.Create(instance); err != nil {
		return nil, err
	}

	bootstrapToken, err := s.issueBootstrapToken(userID, instance.ID)
	if err != nil {
		return nil, err
	}

	summary, err := ToInstanceSummary(*instance)
	if err != nil {
		return nil, err
	}

	return &CreateInstanceResult{
		Instance:  summary,
		Bootstrap: *bootstrapToken,
	}, nil
}

func (s *InstanceService) List(userID string) (*InstanceListResult, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, ErrInvalidInput
	}

	instances, err := s.instances.ListByUser(userID)
	if err != nil {
		return nil, err
	}

	items := make([]InstanceSummary, 0, len(instances))
	for _, instance := range instances {
		summary, err := ToInstanceSummary(instance)
		if err != nil {
			return nil, err
		}
		items = append(items, summary)
	}

	return &InstanceListResult{Items: items}, nil
}

func ToInstanceSummary(instance models.Instance) (InstanceSummary, error) {
	labels, err := decodeStringMapJSON(instance.LabelsJSON)
	if err != nil {
		return InstanceSummary{}, err
	}

	runtimeCapabilities, err := decodeAnyMapJSON(instance.RuntimeCapabilitiesJSON)
	if err != nil {
		return InstanceSummary{}, err
	}

	return InstanceSummary{
		ID:                  instance.ID,
		TargetKind:          "instance",
		Name:                instance.Name,
		PublicIP:            instance.PublicIP,
		PrivateIP:           instance.PrivateIP,
		AgentID:             instance.AgentID,
		Status:              normalizeInstanceStatus(instance.Status),
		Labels:              labels,
		RuntimeCapabilities: runtimeCapabilities,
		CreatedAt:           instance.CreatedAt,
		UpdatedAt:           instance.UpdatedAt,
	}, nil
}

func (s *InstanceService) issueBootstrapToken(userID, instanceID string) (*BootstrapTokenIssue, error) {
	rawToken, err := newOpaqueBootstrapToken()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	expiresAt := now.Add(s.bootstrapTokenTTL())
	record := &models.BootstrapToken{
		ID:         utils.NewPrefixedID("boot"),
		UserID:     userID,
		InstanceID: instanceID,
		TokenHash:  hashOpaqueToken(rawToken),
		ExpiresAt:  expiresAt,
	}
	if err := s.bootstrap.Create(record); err != nil {
		return nil, err
	}

	return &BootstrapTokenIssue{
		Token:     rawToken,
		TokenID:   record.ID,
		ExpiresAt: expiresAt,
		SingleUse: true,
	}, nil
}

func (s *InstanceService) bootstrapTokenTTL() time.Duration {
	if s.enrollment.BootstrapTokenTTL <= 0 {
		return 15 * time.Minute
	}

	return s.enrollment.BootstrapTokenTTL
}

func normalizeInstanceIPs(publicIPInput, privateIPInput string) (*string, *string, error) {
	publicIP, err := normalizeIPAddress(publicIPInput)
	if err != nil {
		return nil, nil, err
	}
	privateIP, err := normalizeIPAddress(privateIPInput)
	if err != nil {
		return nil, nil, err
	}
	if publicIP == nil && privateIP == nil {
		return nil, nil, ErrInvalidInput
	}

	return publicIP, privateIP, nil
}

func normalizeIPAddress(raw string) (*string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}

	ip := net.ParseIP(value)
	if ip == nil {
		return nil, ErrInvalidIP
	}

	normalized := ip.String()
	return &normalized, nil
}

func normalizeInstanceLabels(labels map[string]string) (map[string]string, error) {
	if len(labels) == 0 {
		return map[string]string{}, nil
	}

	normalized := make(map[string]string, len(labels))
	for rawKey, rawValue := range labels {
		key := normalizeInstanceLabelKey(rawKey)
		value := utils.NormalizeSpace(rawValue)
		if key == "" || value == "" || len(value) > 255 {
			return nil, ErrInvalidInput
		}
		if _, exists := normalized[key]; exists {
			return nil, ErrInvalidInput
		}
		normalized[key] = value
	}

	return normalized, nil
}

func normalizeInstanceLabelKey(key string) string {
	key = strings.TrimSpace(strings.ToLower(key))
	if key == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(key))
	lastSeparator := false

	for _, r := range key {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastSeparator = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastSeparator = false
		case r == '-' || r == '_' || r == '.':
			if b.Len() > 0 && !lastSeparator {
				b.WriteRune(r)
				lastSeparator = true
			}
		case unicode.IsSpace(r) || r == '/' || r == ':':
			if b.Len() > 0 && !lastSeparator {
				b.WriteByte('-')
				lastSeparator = true
			}
		default:
			if b.Len() > 0 && !lastSeparator {
				b.WriteByte('-')
				lastSeparator = true
			}
		}
	}

	return strings.Trim(b.String(), "-_.")
}

func normalizeInstanceStatus(status string) string {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "online":
		return "online"
	case "offline":
		return "offline"
	case "degraded":
		return "degraded"
	case "revoked":
		return "revoked"
	default:
		return "pending_enrollment"
	}
}

func decodeStringMapJSON(raw string) (map[string]string, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]string{}, nil
	}

	var out map[string]string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	if out == nil {
		return map[string]string{}, nil
	}

	return out, nil
}

func decodeAnyMapJSON(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}, nil
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	if out == nil {
		return map[string]any{}, nil
	}

	return out, nil
}

func newOpaqueBootstrapToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}

	return "lop_boot_" + base64.RawURLEncoding.EncodeToString(raw), nil
}
