// Package domain 纯领域模型，禁止 import gorm/gin/casbin 等基础设施。
package domain

// TenantType 租户类型
type TenantType string

const (
	// TenantTypePersonal 个人租户：用户注册时自动创建，owner 为用户本人
	TenantTypePersonal TenantType = "personal"
	// TenantTypeOrganization 企业/组织租户：用户主动创建并管理
	TenantTypeOrganization TenantType = "organization"
)

// TenantStatus 租户状态
type TenantStatus string

const (
	TenantStatusActive   TenantStatus = "active"
	TenantStatusDisabled TenantStatus = "disabled"
	TenantStatusSuspended TenantStatus = "suspended"
)

// Tenant 租户：多租户隔离的顶层容器。
// 资源、数据、角色、策略均归属某个租户，租户之间完全隔离。
type Tenant struct {
	ID   uint64
	Type TenantType

	// Name 显示名称
	Name string
	// Slug 唯一标识（URL/资源定位用，如 org.acme）
	Slug string

	Status TenantStatus

	// OwnerUserID 租户所有者（个人租户即用户本人）
	OwnerUserID uint64

	// 配额快照（个人租户通常为默认配额，企业可配置）
	MaxMembers int
	MaxTeams   int

	CreatedAt int64
	UpdatedAt int64
}
