package channel

import (
	"time"
)

// ChannelGroup은 'channel_groups' 테이블의 스키마입니다.
type ChannelGroup struct {
	ID                 uint64    `json:"id" db:"id"`
	ChannelGroupName   string    `json:"channel_group_name" db:"channel_group_name"`
	ChannelGroupDesc   *string   `json:"channel_group_desc" db:"channel_group_desc"` 
	CreatedID          uint64    `json:"created_id" db:"created_id"`
	CreatedByName      string    `json:"created_by_name" db:"user_name"` // (추가)
	CreatedAt          time.Time `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time `json:"updated_at" db:"updated_at"`
}

type ChannelDetail struct {
	ID          uint64    `json:"id" db:"id"`
	ChannelName string    `json:"channel_name" db:"channel_name"`
	ChannelID   string    `json:"channel_id" db:"channel_id"` 
	CreatedID   uint64    `json:"created_id" db:"created_id"`
	CreatedByName    string    `json:"created_by_name" db:"user_name"` // (추가)
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// ChannelGroupMapping은 'channel_group_mapping' 테이블의 스키마입니다.
type ChannelGroupMapping struct {
	ChannelGroupID uint64    `json:"channel_group_id" db:"channel_group_id"`
	ChannelID      uint64    `json:"channel_id" db:"channel_id"`
	CreatedID      int       `json:"created_id" db:"created_id"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
}