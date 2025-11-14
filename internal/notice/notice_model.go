package notice

import (
	"time"
)

// NoticeSchedule은 'notice_schedules' 테이블의 스키마입니다.
type NoticeSchedule struct {
	ID               uint64    `json:"id" db:"id"`
	NoticeTitle      string    `json:"notice_title" db:"notice_title"`
	TemplateID       uint64    `json:"template_id" db:"template_id"`
	MessageType      string    `json:"message_type" db:"message_type"` // (신규 필드)
	ChannelGroupID   uint64    `json:"channel_group_id" db:"channel_group_id"`
	NoticeStartDe    time.Time `json:"notice_start_de" db:"notice_start_de"`
	NoticeEndDe      time.Time `json:"notice_end_de" db:"notice_end_de"`
	NoticeTime       string    `json:"notice_time" db:"notice_time"`
	NoticeInterval   string    `json:"notice_interval" db:"notice_interval"`
	HereYn           bool      `json:"here_yn" db:"here_yn"`
	ChannelYn        bool      `json:"channel_yn" db:"channel_yn"`
	NoticeContents   string    `json:"notice_contents" db:"notice_contents"` // JSON
	SlackbotID       uint64    `json:"slackbot_id" db:"slackbot_id"`
	CreatedID        uint64    `json:"created_id" db:"created_id"`
	CreatedByName    string    `json:"created_by_name" db:"user_name"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}