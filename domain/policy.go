package domain

// Permission 权限点（能力）：平台级定义的原子操作。
// 如 create_team、delete_user、view_billing。所有租户共享同一套能力清单。
type Permission struct {
	// Action 动作（如 create、read、update、delete、manage）
	Action string
	// Resource 资源（如 team、user、role、billing）
	Resource string
	// Name 能力名（Action + ":" + Resource，如 create:team）
	Name string
	// Description 描述
	Description string
}

// Policy 策略：角色 ↔ 权限点的绑定，租户级。
// 每个租户给角色配哪些权限，各自独立。
// 对应 Casbin 规则：p, <roleName>, <tenantID>, <resource>, <action>
type Policy struct {
	ID uint64

	TenantID uint64

	// RoleID 关联的角色（系统或租户角色均可）
	RoleID uint64

	// Action 动作
	Action string
	// Resource 资源
	Resource string

	// Effect 效果：allow / deny（默认 allow）
	Effect string

	CreatedAt int64
	UpdatedAt int64
}

// PermissionName 构造能力名（Action:Resource）
func (p *Policy) PermissionName() string { return p.Action + ":" + p.Resource }
