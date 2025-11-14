package template

import (
	"fmt"
	"log"
	"strings"

	"github.com/go-sql-driver/mysql" // (UNIQUE 에러 확인용)
)

// (MySQL 'Duplicate entry' 에러 코드)
const (
	ErrMySQLDuplicateEntry = 1062
)

// Service는 'template' 기능의 비즈니스 로직을 담당합니다.
type Service struct {
	store *Store
}

// NewService는 새 Service를 생성합니다.
func NewService(store *Store) *Service {
	return &Service{store: store}
}

// GetAllTemplates는 템플릿 목록 조회를 담당합니다.
func (s *Service) GetAllTemplates() ([]Template, error) {
	return s.store.GetAllTemplates()
}

// CreateTemplateRequest는 새 템플릿 생성 폼 데이터입니다.
type CreateTemplateRequest struct {
	TemplateName     string
	TemplateContents string // (Slack Block Kit JSON)
}

// CreateTemplate는 폼 데이터를 모델로 변환하고, 'UNIQUE' 제약 에러를 처리합니다.
func (s *Service) CreateTemplate(req CreateTemplateRequest, createdID uint64) error {
	tmpl := &Template{
		TemplateName:     req.TemplateName,
		TemplateContents: req.TemplateContents,
		CreatedID:        createdID,
	}

	err := s.store.CreateTemplate(tmpl)
	if err != nil {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok {
			if mysqlErr.Number == ErrMySQLDuplicateEntry {
				// (DBA 님: 'templates' 테이블에는 UNIQUE 인덱스가 없어 보이지만,
				//  'template_name'은 중복되지 않는 것이 좋으므로 에러 처리를 추가합니다.)
				if strings.Contains(mysqlErr.Message, "template_name") {
					return fmt.Errorf("이미 존재하는 템플릿명입니다: %s", req.TemplateName)
				}
			}
		}
		log.Printf("[ERROR] CreateTemplate 서비스 에러: %v", err)
		return err
	}
	return nil
}

// GetTemplateByID는 스토어를 호출하여 템플릿을 조회합니다.
func (s *Service) GetTemplateByID(id uint64) (*Template, error) {
	return s.store.GetTemplateByID(id)
}

// UpdateTemplateRequest는 템플릿 수정 폼 데이터입니다.
type UpdateTemplateRequest struct {
	ID               uint64
	TemplateName     string
	TemplateContents string
}

// UpdateTemplate는 템플릿 수정을 처리하고 '권한' 및 'UNIQUE' 에러를 검사합니다.
func (s *Service) UpdateTemplate(req UpdateTemplateRequest, userID uint64, userRole string) error {
	// 1. (권한 확인) 수정을 시도하기 전, DB에서 원본 템플릿 정보를 가져옵니다.
	originalTemplate, err := s.store.GetTemplateByID(req.ID)
	if err != nil {
		return fmt.Errorf("수정할 템플릿(ID: %d)을 찾을 수 없습니다.", req.ID)
	}

	// 2. (권한 부여 로직)
	if userRole != "ADMIN" && originalTemplate.CreatedID != userID {
		return fmt.Errorf("권한 없음: 자신이 작성한 템플릿만 수정할 수 있습니다.")
	}
	
	tmpl := &Template{
		ID:               req.ID,
		TemplateName:     req.TemplateName,
		TemplateContents: req.TemplateContents,
	}

	err = s.store.UpdateTemplate(tmpl)
	if err != nil {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok {
			if mysqlErr.Number == ErrMySQLDuplicateEntry {
				return fmt.Errorf("이미 존재하는 템플릿명입니다: %s", req.TemplateName)
			}
		}
		log.Printf("[ERROR] UpdateTemplate 서비스 에러: %v", err)
		return err
	}
	return nil
}

// DeleteTemplate는 '권한'을 확인한 뒤 템플릿 삭제를 처리합니다.
func (s *Service) DeleteTemplate(id uint64, userID uint64, userRole string) error {
	// 1. (권한 확인) 삭제를 시도하기 전, 원본 템플릿 정보를 가져옵니다.
	originalTemplate, err := s.store.GetTemplateByID(id)
	if err != nil {
		return fmt.Errorf("삭제할 템플릿(ID: %d)을 찾을 수 없습니다.", id)
	}
	
	// 2. (권한 부여 로직)
	if userRole != "ADMIN" && originalTemplate.CreatedID != userID {
		return fmt.Errorf("권한 없음: 자신이 작성한 템플릿만 삭제할 수 있습니다.")
	}

	err = s.store.DeleteTemplate(id)
	if err != nil {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok {
			if mysqlErr.Number == 1451 { // (Foreign Key Constraint Fails)
				return fmt.Errorf("삭제 실패: 이 템플릿을 사용 중인 '공지 스케줄'이 있습니다.")
			}
		}
		return err
	}
	return nil
}