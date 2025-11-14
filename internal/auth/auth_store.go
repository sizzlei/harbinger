package auth

import (
	"database/sql"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/go-sql-driver/mysql"
)

// Store
type Store struct {
	db *sqlx.DB
}

// NewStore
func NewStore(db *sqlx.DB) *Store {
	return &Store{db: db}
}

// CreateUser
func (s *Store) CreateUser(user *User) error {
	query := `
		INSERT INTO users (
			user_name, email, organization, 
			privileges_type, verify_yn, otp_code 
		) VALUES (
			:user_name, :email, :organization, 
			:privileges_type, :verify_yn, :otp_code
		)`
	_, err := s.db.NamedExec(query, user)
	if err != nil {
		log.Printf("[ERROR] CreateUser DB 에러: %v", err)
		return err
	}
	log.Printf("[INFO] 신규 사용자 DB 저장 성공: %s", user.Email)
	return nil
}

// GetUserByEmail
func (s *Store) GetUserByEmail(email string) (*User, error) {
	var user User
	query := `
		SELECT 
			id, user_name, email, organization, 
			otp_code, privileges_type, last_login_dt, 
			verify_yn, created_at, updated_at
		FROM users
		WHERE email = ?
	`
	err := s.db.Get(&user, query, email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil 
		}
		log.Printf("[ERROR] GetUserByEmail DB 에러: %v", err)
		return nil, err
	}
	return &user, nil
}

// UpdateUserOTP
func (s *Store) UpdateUserOTP(email string, otpSecret string) error {
	query := `
		UPDATE users 
		SET 
			otp_code = ?,
			last_login_dt = ? 
		WHERE 
			email = ?`
	_, err := s.db.Exec(query, otpSecret, time.Now(), email)
	if err != nil {
		log.Printf("[ERROR] UpdateUserOTP DB 에러: %v", err)
		return err
	}
	log.Printf("[INFO] 사용자 OTP 코드 저장 성공: %s", email)
	return nil
}

// --- (신규/수정) 관리자 기능 ---

// GetPendingUsers는 승인 대기 중인 사용자 목록을 반환합니다. (기존)
func (s *Store) GetPendingUsers() ([]User, error) {
	var users []User
	query := `
		SELECT id, user_name, email, organization, created_at, verify_yn 
		FROM users 
		WHERE verify_yn = FALSE 
		ORDER BY created_at ASC
	`
	err := s.db.Select(&users, query)
	if err != nil {
		log.Printf("[ERROR] GetPendingUsers DB 에러: %v", err)
		return nil, err
	}
	return users, nil
}

// (신규) GetAllVerifiedUsers는 이미 승인된 사용자 목록을 반환합니다.
func (s *Store) GetAllVerifiedUsers() ([]User, error) {
	var users []User
	query := `
		SELECT id, user_name, email, organization, privileges_type, last_login_dt, verify_yn 
		FROM users 
		WHERE verify_yn = TRUE 
		ORDER BY last_login_dt DESC
	`
	err := s.db.Select(&users, query)
	if err != nil {
		log.Printf("[ERROR] GetAllVerifiedUsers DB 에러: %v", err)
		return nil, err
	}
	return users, nil
}


// ApproveUser는 특정 사용자의 'verify_yn'을 TRUE로 변경합니다. (기존)
func (s *Store) ApproveUser(userID uint64) error {
	query := `UPDATE users SET verify_yn = TRUE WHERE id = ?`
	result, err := s.db.Exec(query, userID)
	if err != nil {
		log.Printf("[ERROR] ApproveUser DB 에러: %v", err)
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return sql.ErrNoRows 
	}
	return nil
}

// (신규) UpdateUserPrivilege는 사용자의 권한('ADMIN' 또는 'USERS')을 변경합니다.
func (s *Store) UpdateUserPrivilege(userID uint64, newRole string) error {
	query := `UPDATE users SET privileges_type = ? WHERE id = ?`
	result, err := s.db.Exec(query, newRole, userID)
	if err != nil {
		log.Printf("[ERROR] UpdateUserPrivilege DB 에러: %v", err)
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return sql.ErrNoRows 
	}
	return nil
}