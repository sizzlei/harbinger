package template

import (
	"time"
)

// Template은 'templates' 테이블의 스키마입니다.
type Template struct {
	ID               uint64    `json:"id" db:"id"`
	TemplateName     string    `json:"template_name" db:"template_name"`
	TemplateContents string    `json:"template_contents" db:"template_contents"` 
	CreatedID        uint64    `json:"created_id" db:"created_id"`
	CreatedByName    string    `json:"created_by_name" db:"user_name"` // (추가)
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}