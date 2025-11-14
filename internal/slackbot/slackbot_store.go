package slackbot

import (
	"log"

	"github.com/jmoiron/sqlx"
	_ "github.com/go-sql-driver/mysql"
)

// Store는 'slackbot' 기능의 DB 로직을 관리합니다.
type Store struct {
	db *sqlx.DB
}

// NewStore는 새 Store를 생성합니다.
func NewStore(db *sqlx.DB) *Store {
	return &Store{db: db}
}

// (수정) 'notice_store.go'에서 이동
// GetBotTokenByID는 ID로 'slackbot_config'에서 봇 토큰을 조회합니다.
func (s *Store) GetBotTokenByID(id uint64) (string, error) {
	var token string
	query := "SELECT bot_token FROM slackbot_config WHERE id = ? AND bot_token IS NOT NULL"
	
	err := s.db.Get(&token, query, id)
	if err != nil {
		log.Printf("[ERROR] GetBotTokenByID DB 에러 (ID: %d): %v", id, err)
		return "", err // (ErrNoRows 포함)
	}
	return token, nil
}

// --- (28단계 신규: 봇 CRUD) ---

// GetAllSlackbots는 모든 봇 목록을 반환합니다 (토큰 제외)
func (s *Store) GetAllSlackbots() ([]SlackbotConfig, error) {
	var bots []SlackbotConfig
	query := `
		SELECT 
			b.id, b.bot_name, b.created_at, b.updated_at, b.created_id,
			u.user_name -- (추가)
		FROM slackbot_config AS b
		JOIN users AS u ON b.created_id = u.id
		ORDER BY b.id DESC
	`
	err := s.db.Select(&bots, query)
	if err != nil {
		log.Printf("[ERROR] GetAllSlackbots DB 에러: %v", err)
		return nil, err
	}
	return bots, nil
}

// GetSlackbotByID는 (수정용) 봇 1개를 조회합니다 (토큰 포함)
func (s *Store) GetSlackbotByID(id uint64) (*SlackbotConfig, error) {
	var bot SlackbotConfig
	query := "SELECT * FROM slackbot_config WHERE id = ?"
	err := s.db.Get(&bot, query, id)
	if err != nil {
		log.Printf("[ERROR] GetSlackbotByID DB 에러: %v", err)
		return nil, err
	}
	return &bot, nil
}

// CreateSlackbot은 새 봇을 DB에 INSERT합니다.
func (s *Store) CreateSlackbot(bot *SlackbotConfig) error {
	query := `
		INSERT INTO slackbot_config (bot_name, bot_token, created_id)
		VALUES (:bot_name, :bot_token, :created_id)
	`
	_, err := s.db.NamedExec(query, bot)
	if err != nil {
		log.Printf("[ERROR] CreateSlackbot DB 에러: %v", err)
		return err
	}
	return nil
}

// UpdateSlackbot은 봇 이름과 토큰을 수정합니다.
func (s *Store) UpdateSlackbot(bot *SlackbotConfig) error {
	query := `
		UPDATE slackbot_config
		SET
			bot_name = :bot_name,
			bot_token = :bot_token
		WHERE
			id = :id
	`
	_, err := s.db.NamedExec(query, bot)
	if err != nil {
		log.Printf("[ERROR] UpdateSlackbot DB 에러: %v", err)
		return err
	}
	return nil
}

// DeleteSlackbot은 ID로 봇을 삭제합니다.
func (s *Store) DeleteSlackbot(id uint64) error {
	query := "DELETE FROM slackbot_config WHERE id = ?"
	_, err := s.db.Exec(query, id)
	if err != nil {
		log.Printf("[ERROR] DeleteSlackbot DB 에러: %v", err)
		return err
	}
	return nil
}