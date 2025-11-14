package template

import (
	"log"

	"github.com/jmoiron/sqlx"
	_ "github.com/go-sql-driver/mysql"
)

// Store는 'template' 기능의 DB 로직을 관리합니다.
type Store struct {
	db *sqlx.DB
}

// NewStore는 새 Store를 생성합니다.
func NewStore(db *sqlx.DB) *Store {
	return &Store{db: db}
}

// CountTemplates는 'templates' 테이블의 총 개수를 반환합니다.
// (대시보드 '등록된 템플릿 수' 용도)
func (s *Store) CountTemplates() (int, error) {
	var count int
	query := "SELECT COUNT(*) FROM templates"
	
	err := s.db.Get(&count, query)
	if err != nil {
		log.Printf("[ERROR] CountTemplates DB 에러: %v", err)
		return 0, err
	}
	
	return count, nil
}

// GetAllTemplates는 모든 템플릿 목록을 반환합니다.
func (s *Store) GetAllTemplates() ([]Template, error) {
	var templates []Template
	query := `
		SELECT 
			t.id, t.template_name, t.created_at, t.updated_at, t.created_id,
			u.user_name -- (추가)
		FROM templates AS t
		JOIN users AS u ON t.created_id = u.id
		ORDER BY t.template_name ASC
	`
	// (성능) 'template_contents' (JSON 본문)는 목록에서 제외
	
	err := s.db.Select(&templates, query)
	if err != nil {
		log.Printf("[ERROR] GetAllTemplates DB 에러: %v", err)
		return nil, err
	}
	return templates, nil
}

// CreateTemplate는 새 템플릿을 DB에 INSERT합니다.
func (s *Store) CreateTemplate(tmpl *Template) error {
	query := `
		INSERT INTO templates (template_name, template_contents, created_id)
		VALUES (:template_name, :template_contents, :created_id)
	`
	_, err := s.db.NamedExec(query, tmpl)
	if err != nil {
		log.Printf("[ERROR] CreateTemplate DB 에러: %v", err)
		return err
	}
	return nil
}

// GetTemplateByID는 ID로 특정 템플릿 1개를 (콘텐츠 포함) 조회합니다.
// (스케줄러가 'template_contents'를 가져오기 위해 사용)
func (s *Store) GetTemplateByID(id uint64) (*Template, error) {
	var tmpl Template
	query := `
		SELECT id, template_name, template_contents, created_id, created_at, updated_at
		FROM templates
		WHERE id = ?
	`
	// (참고: UI 수정용 GetTemplateByID와 동일한 함수입니다. 
	//  이전에 22단계에서 이미 추가했다면, 이 단계는 건너뛰어도 됩니다.)
	err := s.db.Get(&tmpl, query, id)
	if err != nil {
		log.Printf("[ERROR] GetTemplateByID(ID: %d) DB 에러: %v", id, err)
		return nil, err // (ErrNoRows 포함)
	}
	return &tmpl, nil
}

// UpdateTemplate는 템플릿 이름과 내용을 수정합니다.
func (s *Store) UpdateTemplate(tmpl *Template) error {
	query := `
		UPDATE templates
		SET
			template_name = :template_name,
			template_contents = :template_contents
		WHERE
			id = :id
	`
	_, err := s.db.NamedExec(query, tmpl)
	if err != nil {
		log.Printf("[ERROR] UpdateTemplate DB 에러: %v", err)
		return err
	}
	return nil
}

// DeleteTemplate는 ID로 템플릿을 삭제합니다.
func (s *Store) DeleteTemplate(id uint64) error {
	query := "DELETE FROM templates WHERE id = ?"
	_, err := s.db.Exec(query, id)
	if err != nil {
		// (DBA 님 참고: 'notice_schedules'가 'template_id'를 FK로 참조하고 있다면,
		//  이 쿼리는 'Foreign key constraint fails' 에러를 반환할 것입니다.)
		log.Printf("[ERROR] DeleteTemplate DB 에러: %v", err)
		return err
	}
	return nil
}

