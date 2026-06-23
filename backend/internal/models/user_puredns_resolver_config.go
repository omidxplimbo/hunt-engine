package models

import "time"

type UserPureDNSResolverConfig struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	UserID   *uint  `gorm:"index" json:"user_id"`
	OwnerKey string `gorm:"uniqueIndex;not null;size:80" json:"owner_key"`

	ResolversText string `gorm:"type:text" json:"resolvers_text"`
	ResolverCount int    `gorm:"default:0" json:"resolver_count"`

	Enabled bool `gorm:"default:false" json:"enabled"`
}
