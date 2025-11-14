package auth

import (
	"time"
)

// User는 'users' 테이블의 스키마를 Go 코드로 표현합니다.
// (DBA 님이 작성하신 DDL을 기반으로 작성되었습니다 ㅋ ㅋ)
type User struct {
	ID             uint64     `json:"id" db:"id"`                                // bigint UNSIGNED
	UserName       string     `json:"user_name" db:"user_name"`                  // varchar(100)
	Email          string     `json:"email" db:"email"`                          // varchar(150)
	Organization   *string    `json:"organization" db:"organization"`            // varchar(50) NULL
	OtpCode        *string    `json:"otp_code" db:"otp_code"`                    // varchar(30) NULL
	PrivilegesType string     `json:"privileges_type" db:"privileges_type"`      // char(5)
	LastLoginDt    *time.Time `json:"last_login_dt" db:"last_login_dt"`          // datetime(0) NULL
	VerifyYn       bool       `json:"verify_yn" db:"verify_yn"`                  // tinyint(1) (0 or 1)
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`                // datetime(0)
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`                // datetime(0)
}