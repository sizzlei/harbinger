package slackbot

import (
	"database/sql" // (sql.ErrNoRows 확인용)
	"errors"     // (errors.Is 사용)
	"fmt"
	"log"

	"github.com/go-sql-driver/mysql" // (에러 확인용)
)

// (MySQL 'Duplicate entry' 에러 코드)
const (
	ErrMySQLDuplicateEntry = 1062
	ErrMySQLForeignKeyFail = 1451 // (FK 제약 조건 위배)
)

// Service는 'slackbot' 기능의 비즈니스 로직을 담당합니다.
type Service struct {
	store *Store
}

// NewService는 새 Service를 생성합니다.
func NewService(store *Store) *Service {
	return &Service{store: store}
}

// GetAllSlackbots는 스토어를 호출하여 봇 목록을 가져옵니다.
func (s *Service) GetAllSlackbots() ([]SlackbotConfig, error) {
	return s.store.GetAllSlackbots()
}

// GetSlackbotByID는 (수정 페이지용) 스토어를 호출합니다.
func (s *Service) GetSlackbotByID(id uint64) (*SlackbotConfig, error) {
	bot, err := s.store.GetSlackbotByID(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("봇(ID: %d)을 찾을 수 없습니다.", id)
		}
		return nil, err
	}
	return bot, nil
}

// CreateBotRequest는 핸들러가 받는 폼 데이터입니다.
type CreateBotRequest struct {
	BotName  string
	BotToken string
}

// CreateSlackbot은 폼 데이터를 모델로 변환하여 스토어를 호출합니다.
func (s *Service) CreateSlackbot(req CreateBotRequest, createdID uint64) error {
	bot := &SlackbotConfig{
		CreatedID: int(createdID),
	}
	if req.BotName != "" {
		bot.BotName = &req.BotName
	}
	if req.BotToken != "" {
		bot.BotToken = &req.BotToken
	}

	err := s.store.CreateSlackbot(bot)
	if err != nil {
		// (참고: 봇 이름/토큰에 UNIQUE 제약이 있다면 여기서 처리)
		log.Printf("[ERROR] CreateSlackbot 서비스 에러: %v", err)
		return err
	}
	return nil
}

// UpdateBotRequest는 핸들러가 받는 폼 데이터입니다.
type UpdateBotRequest struct {
	ID       uint64
	BotName  string
	BotToken string
}

// (수정) UpdateSlackbot은 '권한' 확인 후 봇을 수정합니다.
func (s *Service) UpdateSlackbot(req UpdateBotRequest, userID uint64, userRole string) error {
	// (신규) 1. 기본 봇(ID=1) 수정 방지
	if req.ID == 1 {
		return fmt.Errorf("권한 없음: 기본 봇(ID: 1)은 수정할 수 없습니다.")
	}

	// 2. (권한 확인) 원본 봇 정보 조회
	originalBot, err := s.store.GetSlackbotByID(req.ID)
	if err != nil {
		return fmt.Errorf("수정할 봇(ID: %d)을 찾을 수 없습니다.", req.ID)
	}

	// 3. (권한 부여 로직)
	// (DBA 님: slackbot_config.created_id는 int 타입, userID는 uint64)
	if userRole != "ADMIN" && uint64(originalBot.CreatedID) != userID {
		return fmt.Errorf("권한 없음: 자신이 등록한 봇만 수정할 수 있습니다.")
	}
	
	bot := &SlackbotConfig{
		ID: req.ID,
	}
	if req.BotName != "" {
		bot.BotName = &req.BotName
	}
	if req.BotToken != "" {
		bot.BotToken = &req.BotToken
	}

	err = s.store.UpdateSlackbot(bot)
	if err != nil {
		log.Printf("[ERROR] UpdateSlackbot 서비스 에러: %v", err)
		return err
	}
	return nil
}

// (수정) DeleteSlackbot은 '권한' 확인 후 봇 삭제를 처리합니다.
func (s *Service) DeleteSlackbot(id uint64, userID uint64, userRole string) error {
	// (신규) 1. 기본 봇(ID=1) 삭제 방지
	if id == 1 {
		return fmt.Errorf("권한 없음: 기본 봇(ID: 1)은 삭제할 수 없습니다.")
	}
	
	// 2. (권한 확인) 원본 봇 정보 조회
	originalBot, err := s.store.GetSlackbotByID(id)
	if err != nil {
		return fmt.Errorf("삭제할 봇(ID: %d)을 찾을 수 없습니다.", id)
	}
	
	// 3. (권한 부여 로직)
	if userRole != "ADMIN" && uint64(originalBot.CreatedID) != userID {
		return fmt.Errorf("권한 없음: 자신이 등록한 봇만 삭제할 수 있습니다.")
	}

	err = s.store.DeleteSlackbot(id)
	if err != nil {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok {
			if mysqlErr.Number == ErrMySQLForeignKeyFail {
				return fmt.Errorf("삭제 실패: 이 봇을 사용 중인 '공지 스케줄'이 있습니다.")
			}
		}
		log.Printf("[ERROR] DeleteSlackbot 서비스 에러: %v", err)
		return err
	}
	return nil
}