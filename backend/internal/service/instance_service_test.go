package service

import (
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"lazyops-server/internal/config"
	"lazyops-server/internal/models"
)

type fakeInstanceStore struct {
	byID       map[string]*models.Instance
	byUserName map[string]*models.Instance
	createErr  error
	listErr    error
	getErr     error
	updateErr  error
}

type fakeBootstrapTokenStore struct {
	byID      map[string]*models.BootstrapToken
	byHash    map[string]*models.BootstrapToken
	createErr error
	lastUsed  map[string]time.Time
}

func newFakeInstanceStore(instances ...*models.Instance) *fakeInstanceStore {
	store := &fakeInstanceStore{
		byID:       make(map[string]*models.Instance),
		byUserName: make(map[string]*models.Instance),
	}

	for _, instance := range instances {
		cloned := *instance
		store.byID[cloned.ID] = &cloned
		store.byUserName[cloned.UserID+":"+cloned.Name] = &cloned
	}

	return store
}

func newFakeBootstrapTokenStore(tokens ...*models.BootstrapToken) *fakeBootstrapTokenStore {
	store := &fakeBootstrapTokenStore{
		byID:     make(map[string]*models.BootstrapToken),
		byHash:   make(map[string]*models.BootstrapToken),
		lastUsed: make(map[string]time.Time),
	}

	for _, token := range tokens {
		cloned := *token
		store.byID[cloned.ID] = &cloned
		store.byHash[cloned.TokenHash] = &cloned
	}

	return store
}

func (f *fakeInstanceStore) Create(instance *models.Instance) error {
	if f.createErr != nil {
		return f.createErr
	}

	cloned := *instance
	now := time.Now().UTC()
	if cloned.CreatedAt.IsZero() {
		cloned.CreatedAt = now
	}
	if cloned.UpdatedAt.IsZero() {
		cloned.UpdatedAt = cloned.CreatedAt
	}
	f.byID[cloned.ID] = &cloned
	f.byUserName[cloned.UserID+":"+cloned.Name] = &cloned
	instance.CreatedAt = cloned.CreatedAt
	instance.UpdatedAt = cloned.UpdatedAt
	return nil
}

func (f *fakeInstanceStore) ListByUser(userID string) ([]models.Instance, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}

	items := make([]models.Instance, 0)
	for _, instance := range f.byID {
		if instance.UserID == userID {
			items = append(items, *instance)
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if !items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].CreatedAt.After(items[j].CreatedAt)
		}
		return items[i].Name < items[j].Name
	})

	return items, nil
}

func (f *fakeInstanceStore) GetByNameForUser(userID, name string) (*models.Instance, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}

	if instance, ok := f.byUserName[userID+":"+name]; ok {
		return instance, nil
	}

	return nil, nil
}

func (f *fakeInstanceStore) GetByIDForUser(userID, instanceID string) (*models.Instance, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}

	instance, ok := f.byID[instanceID]
	if !ok || instance.UserID != userID {
		return nil, nil
	}

	return instance, nil
}

func (f *fakeInstanceStore) GetByID(instanceID string) (*models.Instance, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}

	instance, ok := f.byID[instanceID]
	if !ok {
		return nil, nil
	}

	return instance, nil
}

func (f *fakeInstanceStore) UpdateAgentState(instanceID, agentID, status string, runtimeCapabilitiesJSON *string, at time.Time) (*models.Instance, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}

	instance, ok := f.byID[instanceID]
	if !ok {
		return nil, nil
	}

	if agentID != "" {
		instance.AgentID = ptrString(agentID)
	}
	if status != "" {
		instance.Status = status
	}
	if runtimeCapabilitiesJSON != nil {
		instance.RuntimeCapabilitiesJSON = *runtimeCapabilitiesJSON
	}
	instance.UpdatedAt = at

	return instance, nil
}

func (f *fakeBootstrapTokenStore) Create(token *models.BootstrapToken) error {
	if f.createErr != nil {
		return f.createErr
	}

	cloned := *token
	if cloned.CreatedAt.IsZero() {
		cloned.CreatedAt = time.Now().UTC()
	}
	f.byID[cloned.ID] = &cloned
	f.byHash[cloned.TokenHash] = &cloned
	token.CreatedAt = cloned.CreatedAt
	return nil
}

func (f *fakeBootstrapTokenStore) GetByHash(tokenHash string) (*models.BootstrapToken, error) {
	if token, ok := f.byHash[tokenHash]; ok {
		return token, nil
	}

	return nil, nil
}

func (f *fakeBootstrapTokenStore) MarkUsed(tokenID string, at time.Time) error {
	f.lastUsed[tokenID] = at
	if token, ok := f.byID[tokenID]; ok {
		token.UsedAt = &at
	}
	return nil
}

func testEnrollmentConfig() config.EnrollmentConfig {
	return config.EnrollmentConfig{
		BootstrapTokenTTL: 20 * time.Minute,
	}
}

func TestInstanceServiceRejectsDuplicateNamePerUser(t *testing.T) {
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:        "inst_existing",
		UserID:    "usr_123",
		Name:      "edge-hcm-1",
		Status:    "pending_enrollment",
		CreatedAt: time.Date(2026, 4, 1, 1, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 4, 1, 1, 0, 0, 0, time.UTC),
	})
	tokenStore := newFakeBootstrapTokenStore()
	service := NewInstanceService(instanceStore, tokenStore, testEnrollmentConfig())

	_, err := service.Create(CreateInstanceCommand{
		UserID:   "usr_123",
		Name:     " edge-hcm-1 ",
		PublicIP: "203.0.113.10",
	})
	if !errors.Is(err, ErrInstanceNameExists) {
		t.Fatalf("expected ErrInstanceNameExists, got %v", err)
	}

	result, err := service.Create(CreateInstanceCommand{
		UserID:   "usr_other",
		Name:     "edge-hcm-1",
		PublicIP: "203.0.113.11",
	})
	if err != nil {
		t.Fatalf("create instance for different user: %v", err)
	}
	if result.Instance.Name != "edge-hcm-1" {
		t.Fatalf("expected normalized name, got %q", result.Instance.Name)
	}
}

func TestInstanceServiceRejectsInvalidIP(t *testing.T) {
	instanceStore := newFakeInstanceStore()
	tokenStore := newFakeBootstrapTokenStore()
	service := NewInstanceService(instanceStore, tokenStore, testEnrollmentConfig())

	_, err := service.Create(CreateInstanceCommand{
		UserID:   "usr_123",
		Name:     "edge-hcm-1",
		PublicIP: "999.1.1.1",
	})
	if !errors.Is(err, ErrInvalidIP) {
		t.Fatalf("expected ErrInvalidIP, got %v", err)
	}
}

func TestInstanceServiceIssuesBootstrapToken(t *testing.T) {
	instanceStore := newFakeInstanceStore()
	tokenStore := newFakeBootstrapTokenStore()
	service := NewInstanceService(instanceStore, tokenStore, testEnrollmentConfig())

	before := time.Now().UTC()
	result, err := service.Create(CreateInstanceCommand{
		UserID:    "usr_123",
		Name:      "  edge hcm 1  ",
		PublicIP:  "203.0.113.10",
		PrivateIP: "10.0.0.5",
		Labels: map[string]string{
			"Region ": " sg ",
			"role":    " api ",
		},
	})
	after := time.Now().UTC()
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	if !strings.HasPrefix(result.Instance.ID, "inst_") {
		t.Fatalf("expected inst_ prefix, got %q", result.Instance.ID)
	}
	if result.Instance.TargetKind != "instance" {
		t.Fatalf("expected target kind instance, got %q", result.Instance.TargetKind)
	}
	if result.Instance.Status != "pending_enrollment" {
		t.Fatalf("expected pending_enrollment, got %q", result.Instance.Status)
	}
	if result.Instance.PublicIP == nil || *result.Instance.PublicIP != "203.0.113.10" {
		t.Fatalf("expected canonical public ip, got %#v", result.Instance.PublicIP)
	}
	if result.Instance.PrivateIP == nil || *result.Instance.PrivateIP != "10.0.0.5" {
		t.Fatalf("expected canonical private ip, got %#v", result.Instance.PrivateIP)
	}
	if result.Instance.Labels["region"] != "sg" {
		t.Fatalf("expected normalized region label, got %#v", result.Instance.Labels)
	}
	if result.Instance.Labels["role"] != "api" {
		t.Fatalf("expected normalized role label, got %#v", result.Instance.Labels)
	}
	if len(result.Instance.RuntimeCapabilities) != 0 {
		t.Fatalf("expected empty runtime capabilities, got %#v", result.Instance.RuntimeCapabilities)
	}
	if !strings.HasPrefix(result.Bootstrap.Token, "lop_boot_") {
		t.Fatalf("expected bootstrap token prefix, got %q", result.Bootstrap.Token)
	}
	if !strings.HasPrefix(result.Bootstrap.TokenID, "boot_") {
		t.Fatalf("expected boot_ token id, got %q", result.Bootstrap.TokenID)
	}
	if !result.Bootstrap.SingleUse {
		t.Fatal("expected bootstrap token to be single-use")
	}
	expectedMin := before.Add(testEnrollmentConfig().BootstrapTokenTTL)
	expectedMax := after.Add(testEnrollmentConfig().BootstrapTokenTTL)
	if result.Bootstrap.ExpiresAt.Before(expectedMin) || result.Bootstrap.ExpiresAt.After(expectedMax) {
		t.Fatalf("expected bootstrap expiry within ttl window, got %s", result.Bootstrap.ExpiresAt)
	}

	record, err := tokenStore.GetByHash(hashOpaqueToken(result.Bootstrap.Token))
	if err != nil {
		t.Fatalf("load bootstrap token by hash: %v", err)
	}
	if record == nil {
		t.Fatal("expected hashed bootstrap token record")
	}
	if record.UserID != "usr_123" {
		t.Fatalf("expected token user usr_123, got %q", record.UserID)
	}
	if record.InstanceID != result.Instance.ID {
		t.Fatalf("expected token instance %q, got %q", result.Instance.ID, record.InstanceID)
	}
	if record.TokenHash != hashOpaqueToken(result.Bootstrap.Token) {
		t.Fatal("expected stored hash to match raw bootstrap token")
	}
}

func TestInstanceServiceListScopesToOwner(t *testing.T) {
	instanceStore := newFakeInstanceStore(
		&models.Instance{
			ID:                      "inst_owner",
			UserID:                  "usr_owner",
			Name:                    "owner-instance",
			PublicIP:                ptrString("203.0.113.10"),
			Status:                  "online",
			LabelsJSON:              `{"region":"sg"}`,
			RuntimeCapabilitiesJSON: `{"docker":true}`,
			CreatedAt:               time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC),
			UpdatedAt:               time.Date(2026, 4, 1, 9, 5, 0, 0, time.UTC),
		},
		&models.Instance{
			ID:                      "inst_other",
			UserID:                  "usr_other",
			Name:                    "other-instance",
			PublicIP:                ptrString("203.0.113.11"),
			Status:                  "offline",
			LabelsJSON:              `{"region":"hn"}`,
			RuntimeCapabilitiesJSON: `{"docker":true}`,
			CreatedAt:               time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC),
			UpdatedAt:               time.Date(2026, 4, 1, 10, 5, 0, 0, time.UTC),
		},
	)
	tokenStore := newFakeBootstrapTokenStore()
	service := NewInstanceService(instanceStore, tokenStore, testEnrollmentConfig())

	result, err := service.List("usr_owner")
	if err != nil {
		t.Fatalf("list instances: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected one instance, got %d", len(result.Items))
	}
	if result.Items[0].ID != "inst_owner" {
		t.Fatalf("expected owner-scoped instance, got %q", result.Items[0].ID)
	}
	if result.Items[0].Status != "online" {
		t.Fatalf("expected online status, got %q", result.Items[0].Status)
	}
	if result.Items[0].RuntimeCapabilities["docker"] != true {
		t.Fatalf("expected runtime capabilities to be decoded, got %#v", result.Items[0].RuntimeCapabilities)
	}
}

func ptrString(value string) *string {
	return &value
}
