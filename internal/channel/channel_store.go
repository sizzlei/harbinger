package channel

import (
	"database/sql" // (sql.ErrNoRows 확인용)
	"log"
	"strings"

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

// CountChannelGroups
func (s *Store) CountChannelGroups() (int, error) {
	var count int
	query := "SELECT COUNT(*) FROM channel_groups"
	err := s.db.Get(&count, query)
	if err != nil {
		log.Printf("[ERROR] CountChannelGroups DB 에러: %v", err)
		return 0, err
	}
	return count, nil
}

// GetAllChannelGroups
func (s *Store) GetAllChannelGroups() ([]ChannelGroup, error) {
	var groups []ChannelGroup
	query := `
		SELECT 
			g.id, g.channel_group_name, g.channel_group_desc, g.created_at, g.updated_at, g.created_id,
			u.user_name
		FROM channel_groups AS g
		JOIN users AS u ON g.created_id = u.id
		ORDER BY g.channel_group_name ASC
	`
	err := s.db.Select(&groups, query)
	if err != nil {
		log.Printf("[ERROR] GetAllChannelGroups DB 에러: %v", err)
		return nil, err
	}
	return groups, nil
}

// GetAllChannelDetails
func (s *Store) GetAllChannelDetails() ([]ChannelDetail, error) {
	var details []ChannelDetail
	query := `
		SELECT 
			d.id, d.channel_name, d.channel_id, d.created_at, d.updated_at, d.created_id,
			u.user_name
		FROM channel_details AS d
		JOIN users AS u ON d.created_id = u.id
		ORDER BY d.channel_name ASC
	`
	err := s.db.Select(&details, query)
	if err != nil {
		log.Printf("[ERROR] GetAllChannelDetails DB 에러: %v", err)
		return nil, err
	}
	return details, nil
}

// GetMappedDetailIDs
func (s *Store) GetMappedDetailIDs(groupID uint64) (map[uint64]bool, error) {
	var ids []uint64
	query := "SELECT channel_id FROM channel_group_mapping WHERE channel_group_id = ?"
	err := s.db.Select(&ids, query, groupID)
	if err != nil {
		log.Printf("[ERROR] GetMappedDetailIDs DB 에러: %v", err)
		return nil, err
	}
	idMap := make(map[uint64]bool)
	for _, id := range ids {
		idMap[id] = true
	}
	return idMap, nil
}

// (수정) 스케줄러가 사용하는 함수 (복원)
// GetSlackIDsByGroupID는 'channel_group_id'에 매핑된 
// 'channel_details.channel_id' (실제 Slack ID) 목록을 반환합니다.
func (s *Store) GetSlackIDsByGroupID(groupID uint64) ([]string, error) {
	var slackIDs []string
	
	query := `
		SELECT
			d.channel_id
		FROM
			channel_group_mapping AS m
		JOIN
			channel_details AS d ON m.channel_id = d.id
		WHERE
			m.channel_group_id = ?
	`
	
	err := s.db.Select(&slackIDs, query, groupID)
	if err != nil {
		log.Printf("[ERROR] GetSlackIDsByGroupID DB 에러 (GroupID: %d): %v", groupID, err)
		return nil, err
	}

	if len(slackIDs) == 0 {
		log.Printf("[WARN] [Scheduler] 그룹(ID: %d)에 매핑된 채널이 없습니다.", groupID)
	}
	
	return slackIDs, nil
}


// CreateChannelGroup
func (s *Store) CreateChannelGroup(group *ChannelGroup) error {
	query := `
		INSERT INTO channel_groups (channel_group_name, channel_group_desc, created_id)
		VALUES (:channel_group_name, :channel_group_desc, :created_id)
	`
	_, err := s.db.NamedExec(query, group)
	if err != nil {
		log.Printf("[ERROR] CreateChannelGroup DB 에러: %v", err)
		return err
	}
	return nil
}

// CreateChannelDetail
func (s *Store) CreateChannelDetail(detail *ChannelDetail) error {
	query := `
		INSERT INTO channel_details (channel_name, channel_id, created_id)
		VALUES (:channel_name, :channel_id, :created_id)
	`
	_, err := s.db.NamedExec(query, detail)
	if err != nil {
		log.Printf("[ERROR] CreateChannelDetail DB 에러: %v", err)
		return err
	}
	return nil
}

// UpdateMappings
func (s *Store) UpdateMappings(groupID uint64, detailIDs []uint64, createdID uint64) error {
	tx, err := s.db.Beginx() 
	if err != nil {
		log.Printf("[ERROR] UpdateMappings 트랜잭션 시작 실패: %v", err)
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec("DELETE FROM channel_group_mapping WHERE channel_group_id = ?", groupID)
	if err != nil {
		log.Printf("[ERROR] UpdateMappings DELETE 실패: %v", err)
		return err
	}

	if len(detailIDs) > 0 {
		query := "INSERT INTO channel_group_mapping (channel_group_id, channel_id, created_id) VALUES "
		var args []interface{}
		var valueStrings []string

		for _, detailID := range detailIDs {
			valueStrings = append(valueStrings, "(?, ?, ?)")
			args = append(args, groupID, detailID, createdID)
		}
		query = query + strings.Join(valueStrings, ",")
		_, err = tx.Exec(query, args...)
		if err != nil {
			log.Printf("[ERROR] UpdateMappings Bulk INSERT 실패: %v", err)
			return err
		}
	}
	return tx.Commit()
}

// GetChannelGroupByID
func (s *Store) GetChannelGroupByID(id uint64) (*ChannelGroup, error) {
	var group ChannelGroup
	query := "SELECT * FROM channel_groups WHERE id = ?"
	err := s.db.Get(&group, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		log.Printf("[ERROR] GetChannelGroupByID DB 에러: %v", err)
		return nil, err
	}
	return &group, nil
}

// GetChannelDetailByID
func (s *Store) GetChannelDetailByID(id uint64) (*ChannelDetail, error) {
	var detail ChannelDetail
	query := "SELECT * FROM channel_details WHERE id = ?"
	err := s.db.Get(&detail, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		log.Printf("[ERROR] GetChannelDetailByID DB 에러: %v", err)
		return nil, err
	}
	return &detail, nil
}

// UpdateChannelGroup
func (s *Store) UpdateChannelGroup(group *ChannelGroup) error {
	query := `
		UPDATE channel_groups
		SET channel_group_name = :channel_group_name, channel_group_desc = :channel_group_desc
		WHERE id = :id
	`
	_, err := s.db.NamedExec(query, group)
	if err != nil {
		log.Printf("[ERROR] UpdateChannelGroup DB 에러: %v", err)
		return err
	}
	return nil
}

// (수정) DeleteChannelGroup은 트랜잭션을 사용해 '매핑'과 '그룹'을 모두 삭제합니다.
func (s *Store) DeleteChannelGroup(id uint64) error {
	// 1. 트랜잭션 시작
	tx, err := s.db.Beginx()
	if err != nil {
		log.Printf("[ERROR] DeleteChannelGroup 트랜잭션 시작 실패: %v", err)
		return err
	}
	defer tx.Rollback() // (기본은 롤백)

	// 2. (신규) 'channel_group_mapping' 테이블에서 먼저 삭제 (CASCADE)
	_, err = tx.Exec("DELETE FROM channel_group_mapping WHERE channel_group_id = ?", id)
	if err != nil {
		log.Printf("[ERROR] DeleteChannelGroup (매핑 삭제) 실패: %v", err)
		return err
	}
	
	// 3. 'channel_groups' 테이블에서 그룹 삭제
	// (만약 'notice_schedules'가 이 ID를 사용 중이면,
	//  FK(RESTRICT)에 의해 이 쿼리가 실패할 것입니다.)
	_, err = tx.Exec("DELETE FROM channel_groups WHERE id = ?", id)
	if err != nil {
		log.Printf("[ERROR] DeleteChannelGroup (그룹 삭제) 실패: %v", err)
		return err
	}

	// 4. 모든 삭제 성공 시 커밋
	return tx.Commit()
}

// UpdateChannelDetail
func (s *Store) UpdateChannelDetail(detail *ChannelDetail) error {
	query := `
		UPDATE channel_details
		SET channel_name = :channel_name, channel_id = :channel_id
		WHERE id = :id
	`
	_, err := s.db.NamedExec(query, detail)
	if err != nil {
		log.Printf("[ERROR] UpdateChannelDetail DB 에러: %v", err)
		return err
	}
	return nil
}

// DeleteChannelDetail
func (s *Store) DeleteChannelDetail(id uint64) error {
	query := "DELETE FROM channel_details WHERE id = ?"
	_, err := s.db.Exec(query, id)
	if err != nil {
		log.Printf("[ERROR] DeleteChannelDetail DB 에러: %v", err)
		return err
	}
	return nil
}