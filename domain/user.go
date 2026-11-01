package domain

// UserStatus 用户状态
type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusDisabled UserStatus = "disabled"
	UserStatusLocked   UserStatus = "locked"
)

// User 用户：登录主体，与租户为多对多关系（经 Membership 连接）。
// 一个用户可属于多个租户，在每个租户内有独立角色。
type User struct {
	ID uint64

	// Username 用户名（唯一，登录标识之一）
	Username string
	// Email 邮箱（唯一，登录标识之一，可为空）
	Email string
	// Phone 手机号（可为空）
	Phone string

	// PasswordHash 密码哈希（bcrypt），OAuth 登录用户可为空
	PasswordHash string

	// DisplayName 显示名
	DisplayName string
	// AvatarURL 头像
	AvatarURL string

	Status UserStatus

	CreatedAt int64
	UpdatedAt int64
}
