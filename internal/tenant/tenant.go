package tenant

import (
	"context"
	"errors"
)

// Tenant represents a tenant in the system.
type Tenant struct {
	ID       string
	Name     string
	Email    string
	Plan     string // free, pro, enterprise
	Status   string // active, suspended, inactive
	Metadata map[string]string
}

// Context key for storing tenant in request context
type contextKey string

const tenantContextKey contextKey = "tenant"

// ErrNoTenant is returned when no tenant is found in context
var ErrNoTenant = errors.New("no tenant in context")

// ErrInvalidTenant is returned when tenant ID is invalid
var ErrInvalidTenant = errors.New("invalid tenant ID")

// WithTenant stores a tenant in the context
func WithTenant(ctx context.Context, tenant *Tenant) context.Context {
	return context.WithValue(ctx, tenantContextKey, tenant)
}

// FromContext retrieves a tenant from the context
func FromContext(ctx context.Context) (*Tenant, error) {
	tenant, ok := ctx.Value(tenantContextKey).(*Tenant)
	if !ok {
		return nil, ErrNoTenant
	}
	return tenant, nil
}

// Store interface for managing tenants (to be implemented)
type Store interface {
	GetTenant(id string) (*Tenant, error)
	CreateTenant(tenant *Tenant) error
	UpdateTenant(tenant *Tenant) error
	DeleteTenant(id string) error
	ListTenants() ([]*Tenant, error)
}

// MockStore is a simple in-memory tenant store for testing
type MockStore struct {
	tenants map[string]*Tenant
}

// NewMockStore creates a new mock tenant store
func NewMockStore() *MockStore {
	return &MockStore{
		tenants: make(map[string]*Tenant),
	}
}

// GetTenant retrieves a tenant by ID
func (m *MockStore) GetTenant(id string) (*Tenant, error) {
	if id == "" {
		return nil, ErrInvalidTenant
	}
	tenant, ok := m.tenants[id]
	if !ok {
		return nil, ErrNoTenant
	}
	return tenant, nil
}

// CreateTenant creates a new tenant
func (m *MockStore) CreateTenant(tenant *Tenant) error {
	if tenant.ID == "" {
		return ErrInvalidTenant
	}
	if _, ok := m.tenants[tenant.ID]; ok {
		return errors.New("tenant already exists")
	}
	m.tenants[tenant.ID] = tenant
	return nil
}

// UpdateTenant updates an existing tenant
func (m *MockStore) UpdateTenant(tenant *Tenant) error {
	if tenant.ID == "" {
		return ErrInvalidTenant
	}
	if _, ok := m.tenants[tenant.ID]; !ok {
		return ErrNoTenant
	}
	m.tenants[tenant.ID] = tenant
	return nil
}

// DeleteTenant deletes a tenant
func (m *MockStore) DeleteTenant(id string) error {
	if id == "" {
		return ErrInvalidTenant
	}
	delete(m.tenants, id)
	return nil
}

// ListTenants lists all tenants
func (m *MockStore) ListTenants() ([]*Tenant, error) {
	tenants := make([]*Tenant, 0, len(m.tenants))
	for _, t := range m.tenants {
		tenants = append(tenants, t)
	}
	return tenants, nil
}
