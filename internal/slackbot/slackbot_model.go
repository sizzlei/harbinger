package slackbot

import (
	"time"
)

// SlackbotConfig는 'slackbot_config' 테이블의 스키마입니다.
type SlackbotConfig struct {
	ID        uint64    `json:"id" db:"id"`
	BotName   *string   `json:"bot_name" db:"bot_name"`
	BotToken  *string   `json:"bot_token" db:"bot_token"` 
	CreatedID int       `json:"created_id" db:"created_id"`
	CreatedByName string    `json:"created_by_name" db:"user_name"` // (추가)
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}