package domain

// Team 团队：租户内部的组织分组，用于数据范围授权。
// 用户可属于租户内多个团队。
type Team struct {
	ID uint64

	TenantID uint64

	// Name 团队名
	Name string
	// Slug 团队唯一标识（租户内唯一）
	Slug string

	// LeaderUserID 团队负责人（可选）
	LeaderUserID uint64

	CreatedAt int64
	UpdatedAt int64
}

// TeamMembership 用户与团队的从属关系（租户内）。
type TeamMembership struct {
	ID uint64

	TeamID uint64
	UserID uint64
	TenantID uint64

	CreatedAt int64
}
