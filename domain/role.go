package domain

// RoleScope 角色作用域
type RoleScope string

const (
	// RoleScopeSystem 平台内置角色，全局定义（owner/admin/member 等）
	RoleScopeSystem RoleScope = "system"
	// RoleScopeTenant 租户自定义角色
	RoleScopeTenant RoleScope = "tenant"
)

// Role 角色：一组权限点的集合。
// 平台提供内置角色（system），租户可自定义角色（tenant）。
// 角色的权限绑定由 Policy 描述（见 policy.go）。
type Role struct {
	ID uint64

	// Scope 角色作用域
	Scope RoleScope
	// TenantID 租户 ID（Scope=tenant 时必填，system 时忽略）
	TenantID uint64

	// Name 角色名（如 admin、member、sales_manager）
	Name string
	// DisplayName 展示名（如 管理员、销售主管）
	DisplayName string
	// Description 描述
	Description string

	CreatedAt int64
	UpdatedAt int64
}
