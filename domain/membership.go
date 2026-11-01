package domain

// MembershipStatus 成员关系状态
type MembershipStatus string

const (
	MembershipStatusActive   MembershipStatus = "active"
	MembershipStatusInvited  MembershipStatus = "invited"  // 已邀请未接受
	MembershipStatusDisabled MembershipStatus = "disabled" // 被移出/禁用
)

// Membership 用户与租户的成员关系，携带该用户在租户内的角色。
// User ↔ Tenant 多对多的中间实体。
type Membership struct {
	ID uint64

	UserID   uint64
	TenantID uint64

	// RoleID 用户在租户内扮演的角色（可自定义角色的 ID）
	RoleID uint64

	// IsOwner 是否为租户所有者（冗余便捷标记，与 Role 保持一致性由 service 保证）
	IsOwner bool

	Status MembershipStatus

	CreatedAt int64
	UpdatedAt int64
}
