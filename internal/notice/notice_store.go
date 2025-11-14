package notice

import (
	"log"
	// "time"

	"github.com/jmoiron/sqlx"
	_ "github.com/go-sql-driver/mysql"
)

// Store는 'notice' 기능의 DB 로직을 관리합니다.
type Store struct {
	db *sqlx.DB
}

// NewStore는 새 Store를 생성합니다.
func NewStore(db *sqlx.DB) *Store {
	return &Store{db: db}
}

// GetActiveNotices는 활성화된 공지 목록을 반환합니다.
func (s *Store) GetActiveNotices(userID uint64, userRole string) ([]NoticeSchedule, error) {
	var notices []NoticeSchedule
	var args []interface{} // (동적 쿼리를 위한 인자)

	// (수정) 기본 쿼리 (JOIN users)
	query := `
		SELECT 
			ns.id, ns.notice_title, ns.template_id, ns.message_type, ns.channel_group_id, 
			ns.notice_start_de, ns.notice_end_de, ns.notice_time, 
			ns.notice_interval, ns.here_yn, ns.channel_yn, 
			ns.notice_contents, ns.slackbot_id, 
			ns.created_id, ns.created_at, ns.updated_at,
			u.user_name
		FROM 
			notice_schedules AS ns
		JOIN 
			users AS u ON ns.created_id = u.id
		WHERE 
			ns.notice_end_de >= CURDATE()
	`

	// (수정) 권한 부여 로직: USERS일 경우, created_id 조건 추가
	if userRole != "ADMIN" {
		query += " AND ns.created_id = ? "
		args = append(args, userID)
	}

	query += " ORDER BY ns.notice_end_de ASC "
	
	// (수정) sqlx.Select는 동적 인자를 받음
	err := s.db.Select(&notices, query, args...)
	if err != nil {
		log.Printf("[ERROR] GetActiveNotices DB 에러: %v", err)
		return nil, err
	}
	
	return notices, nil
}

// GetNoticeScheduleByID는 ID로 특정 공지 1개를 (콘텐츠 포함) 조회합니다. (수정용)
func (s *Store) GetNoticeScheduleByID(id uint64) (*NoticeSchedule, error) {
	var ns NoticeSchedule
	query := `
		SELECT 
			id, notice_title, template_id, message_type, channel_group_id, 
			notice_start_de, notice_end_de, notice_time, 
			notice_interval, here_yn, channel_yn, 
			notice_contents, slackbot_id, 
			created_id, created_at, updated_at
		FROM notice_schedules
		WHERE id = ?
	`
	err := s.db.Get(&ns, query, id)
	if err != nil {
		log.Printf("[ERROR] GetNoticeScheduleByID DB 에러: %v", err)
		return nil, err // (ErrNoRows 포함)
	}
	return &ns, nil
}

// CreateNoticeSchedule은 새 공지 스케줄을 DB에 INSERT합니다. (생성용)
func (s *Store) CreateNoticeSchedule(ns *NoticeSchedule) error {
	query := `
		INSERT INTO notice_schedules (
			notice_title, template_id, message_type, channel_group_id, 
			notice_start_de, notice_end_de, notice_time, 
			notice_interval, here_yn, channel_yn, 
			notice_contents, slackbot_id, created_id
		) VALUES (
			:notice_title, :template_id, :message_type, :channel_group_id, 
			:notice_start_de, :notice_end_de, :notice_time, 
			:notice_interval, :here_yn, :channel_yn, 
			:notice_contents, :slackbot_id, :created_id
		)
	`
	_, err := s.db.NamedExec(query, ns)
	if err != nil {
		log.Printf("[ERROR] CreateNoticeSchedule DB 에러: %v", err)
		return err
	}
	return nil
}

// UpdateNoticeSchedule는 공지 스케줄 내용을 수정합니다. (수정용)
func (s *Store) UpdateNoticeSchedule(ns *NoticeSchedule) error {
	query := `
		UPDATE notice_schedules
		SET
			notice_title = :notice_title,
			template_id = :template_id,
			message_type = :message_type, 
			channel_group_id = :channel_group_id,
			notice_start_de = :notice_start_de,
			notice_end_de = :notice_end_de,
			notice_time = :notice_time,
			notice_interval = :notice_interval,
			here_yn = :here_yn,
			channel_yn = :channel_yn,
			notice_contents = :notice_contents,
			slackbot_id = :slackbot_id
		WHERE
			id = :id
	`
	_, err := s.db.NamedExec(query, ns)
	if err != nil {
		log.Printf("[ERROR] UpdateNoticeSchedule DB 에러: %v", err)
		return err
	}
	return nil
}

// DeleteNoticeSchedule는 ID로 공지를 삭제합니다. (삭제용)
func (s *Store) DeleteNoticeSchedule(id uint64) error {
	query := "DELETE FROM notice_schedules WHERE id = ?"
	_, err := s.db.Exec(query, id)
	if err != nil {
		log.Printf("[ERROR] DeleteNoticeSchedule DB 에러: %v", err)
		return err
	}
	return nil
}

// GetNoticesToRunNow는 스케줄러가 '지금' 발송해야 할 공지 목록을 반환합니다.
func (s *Store) GetNoticesToRunNow() ([]NoticeSchedule, error) {
	var notices []NoticeSchedule

	// (DBA 님) 이 쿼리가 스케줄러의 핵심입니다.
	// 1. 날짜가 유효하고 (시작일 <= 오늘 <= 종료일)
	// 2. 시간이 유효하고 (공지 시간(HH:MM) == 현재 시간(HH:MM))
	// 3. (임시) 간격이 'daily'인 공지를 찾습니다.
	query := `
		SELECT
			id, notice_title, template_id, message_type, channel_group_id, 
			notice_start_de, notice_end_de, notice_time, 
			notice_interval, here_yn, channel_yn, 
			notice_contents, slackbot_id, 
			created_id, created_at, updated_at
		FROM 
			notice_schedules
		WHERE 
			-- 1. 날짜 범위 확인 (인덱스 'idx_notice_schedules_01' 활용)
			(CURDATE() BETWEEN notice_start_de AND notice_end_de)
		AND 
			-- 2. 시간(분) 확인 (예: "09:30" == "09:30")
			(TIME_FORMAT(notice_time, '%H:%i') = TIME_FORMAT(NOW(), '%H:%i'))
		AND
			-- 3. (수정) 간격 확인: (오늘 - 시작일) % 간격 == 0
			-- (CAST로 varchar '3'을 숫자 3으로 변환. 0이나 문자로 인한 오류 방지)
			(
				CAST(notice_interval AS UNSIGNED) > 0 
			AND 
				DATEDIFF(CURDATE(), notice_start_de) % CAST(notice_interval AS UNSIGNED) = 0
			)
	`
	
	err := s.db.Select(&notices, query)
	if err != nil {
		// (참고: 'sql.ErrNoRows'는 Select에서 에러가 아님. 빈 슬라이스 반환)
		log.Printf("[ERROR] [Scheduler] GetNoticesToRunNow DB 에러: %v", err)
		return nil, err
	}
	
	return notices, nil
}

