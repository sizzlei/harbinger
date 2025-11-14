package channel

import (
	"database/sql"
	"errors" // (errors.Is를 위해 임포트)
	"fmt"
	"log"
	"strings" // (MySQL 에러 메시지 확인용)

	"github.com/go-sql-driver/mysql" // (MySQL 에러 코드 확인용)
	"golang.org/x/sync/errgroup"
)

// (MySQL 'Duplicate entry' 에러 코드)
const (
	ErrMySQLDuplicateEntry = 1062
	ErrMySQLForeignKeyFail = 1451 // (FK 제약 조건 위배)
)

// Service는 'channel' 기능의 비즈니스 로직을 담당합니다.
type Service struct {
	store *Store
}

// NewService는 새 Service를 생성합니다.
func NewService(store *Store) *Service {
	return &Service{store: store}
}

// ListPageData는 채널 관리 페이지에 필요한 모든 데이터를 병렬로 조회합니다.
type ListPageData struct {
	Groups          []ChannelGroup
	Details         []ChannelDetail
	MappedDetailIDs map[uint64]bool // (Key: DetailID, Value: true)
	SelectedGroupID uint64          // (현재 선택된 그룹 ID)
}

// GetChannelListPageData는 채널 관리 페이지 데이터를 조회합니다.
func (s *Service) GetChannelListPageData(selectedGroupID uint64) (*ListPageData, error) {
	var data ListPageData
	data.SelectedGroupID = selectedGroupID
	var eg errgroup.Group

	// 고루틴 1: 그룹 목록 조회
	eg.Go(func() error {
		groups, err := s.store.GetAllChannelGroups()
		if err != nil {
			log.Printf("[ERROR] GetChannelListPageData: GetAllChannelGroups 실패: %v", err)
			return err
		}
		data.Groups = groups
		return nil
	})

	// 고루틴 2: 상세 채널 목록 조회
	eg.Go(func() error {
		details, err := s.store.GetAllChannelDetails()
		if err != nil {
			log.Printf("[ERROR] GetChannelListPageData: GetAllChannelDetails 실패: %v", err)
			return err
		}
		data.Details = details
		return nil
	})

	// 고루틴 3: (선택 사항) 그룹이 선택되었으면, 매핑된 ID 목록 조회
	if selectedGroupID > 0 {
		eg.Go(func() error {
			mappedIDs, err := s.store.GetMappedDetailIDs(selectedGroupID)
			if err != nil {
				log.Printf("[ERROR] GetChannelListPageData: GetMappedDetailIDs 실패: %v", err)
				return err
			}
			data.MappedDetailIDs = mappedIDs
			return nil
		})
	} else {
		data.MappedDetailIDs = make(map[uint64]bool)
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return &data, nil
}

// CreateGroupRequest는 새 그룹 생성 시 핸들러가 받는 폼 데이터입니다.
type CreateGroupRequest struct {
	GroupName string
	GroupDesc string
}

// CreateChannelGroup은 폼 데이터를 모델로 변환하여 스토어를 호출합니다.
func (s *Service) CreateChannelGroup(req CreateGroupRequest, createdID uint64) error {
	group := &ChannelGroup{
		ChannelGroupName: req.GroupName,
		CreatedID:        createdID,
	}
	if req.GroupDesc != "" {
		group.ChannelGroupDesc = &req.GroupDesc
	}

	err := s.store.CreateChannelGroup(group)
	if err != nil {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok {
			if mysqlErr.Number == ErrMySQLDuplicateEntry {
				return fmt.Errorf("이미 존재하는 그룹명입니다: %s", req.GroupName)
			}
		}
	}
	return err
}

// CreateDetailRequest는 새 상세 채널 생성 시 핸들러가 받는 폼 데이터입니다.
type CreateDetailRequest struct {
	ChannelName string
	ChannelID   string
}

// CreateChannelDetail은 폼 데이터를 모델로 변환하여 스토어를 호출합니다.
func (s *Service) CreateChannelDetail(req CreateDetailRequest, createdID uint64) error {
	detail := &ChannelDetail{
		ChannelName: req.ChannelName,
		ChannelID:   req.ChannelID,
		CreatedID:   createdID,
	}

	err := s.store.CreateChannelDetail(detail)
	if err != nil {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok {
			if mysqlErr.Number == ErrMySQLDuplicateEntry {
				if strings.Contains(mysqlErr.Message, "udx_channel_details_01") {
					return fmt.Errorf("이미 존재하는 채널명입니다: %s", req.ChannelName)
				}
				if strings.Contains(mysqlErr.Message, "udx_channel_details_02") {
					return fmt.Errorf("이미 등록된 Slack 채널 ID입니다: %s", req.ChannelID)
				}
			}
		}
	}
	return err
}

// UpdateGroupMappings에 '권한' 확인 로직 추가
func (s *Service) UpdateGroupMappings(groupID uint64, detailIDs []uint64, userID uint64, userRole string) error {
	if groupID == 0 {
		return fmt.Errorf("매핑할 그룹이 선택되지 않았습니다.")
	}

	// 1. (권한 확인) 이 그룹의 원본 정보를 가져옵니다.
	originalGroup, err := s.store.GetChannelGroupByID(groupID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("매핑할 그룹(ID: %d)을 찾을 수 없습니다.", groupID)
		}
		return err
	}

	// 2. (권한 부여 로직)
	if userRole != "ADMIN" && originalGroup.CreatedID != userID {
		return fmt.Errorf("권한 없음: 자신이 생성한 그룹의 매핑만 수정할 수 있습니다.")
	}

	return s.store.UpdateMappings(groupID, detailIDs, userID)
}

// --- (QA 항목 3) ---

// GetChannelGroupByID는 (수정 페이지용) 스토어를 호출합니다.
func (s *Service) GetChannelGroupByID(id uint64) (*ChannelGroup, error) {
	group, err := s.store.GetChannelGroupByID(id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("그룹(ID: %d)을 찾을 수 없습니다.", id)
	}
	return group, err
}

// GetChannelDetailByID는 (수정 페이지용) 스토어를 호출합니다.
func (s *Service) GetChannelDetailByID(id uint64) (*ChannelDetail, error) {
	detail, err := s.store.GetChannelDetailByID(id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("상세 채널(ID: %d)을 찾을 수 없습니다.", id)
	}
	return detail, err
}

// UpdateChannelGroup은 '권한' 확인 후 그룹을 수정합니다.
func (s *Service) UpdateChannelGroup(req CreateGroupRequest, groupID uint64, userID uint64, userRole string) error {
	originalGroup, err := s.store.GetChannelGroupByID(groupID)
	if err != nil {
		return fmt.Errorf("수정할 그룹(ID: %d)을 찾을 수 없습니다.", groupID)
	}
	if userRole != "ADMIN" && originalGroup.CreatedID != userID {
		return fmt.Errorf("권한 없음: 자신이 생성한 그룹만 수정할 수 있습니다.")
	}

	group := &ChannelGroup{
		ID:               groupID,
		ChannelGroupName: req.GroupName,
	}
	if req.GroupDesc != "" {
		group.ChannelGroupDesc = &req.GroupDesc
	}

	err = s.store.UpdateChannelGroup(group)
	if err != nil {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok {
			if mysqlErr.Number == ErrMySQLDuplicateEntry {
				return fmt.Errorf("이미 존재하는 그룹명입니다: %s", req.GroupName)
			}
		}
		return err
	}
	return nil
}

// DeleteChannelGroup은 '권한' 확인 후 그룹을 삭제합니다.
func (s *Service) DeleteChannelGroup(groupID uint64, userID uint64, userRole string) error {
	originalGroup, err := s.store.GetChannelGroupByID(groupID)
	if err != nil {
		return fmt.Errorf("삭제할 그룹(ID: %d)을 찾을 수 없습니다.", groupID)
	}
	if userRole != "ADMIN" && originalGroup.CreatedID != userID {
		return fmt.Errorf("권한 없음: 자신이 생성한 그룹만 삭제할 수 있습니다.")
	}

	err = s.store.DeleteChannelGroup(groupID)
	if err != nil {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok {
			// (수정) 매핑 FK 에러는 스토어에서 처리되므로, '공지 스케줄' FK 에러만 확인
			if mysqlErr.Number == ErrMySQLForeignKeyFail {
				return fmt.Errorf("삭제 실패: 이 그룹을 사용 중인 '공지 스케줄'이 있습니다.")
			}
		}
		return err
	}
	return nil
}

// UpdateChannelDetail은 '권한' 확인 후 상세 채널을 수정합니다.
func (s *Service) UpdateChannelDetail(req CreateDetailRequest, detailID uint64, userID uint64, userRole string) error {
	originalDetail, err := s.store.GetChannelDetailByID(detailID)
	if err != nil {
		return fmt.Errorf("수정할 상세 채널(ID: %d)을 찾을 수 없습니다.", detailID)
	}
	if userRole != "ADMIN" && originalDetail.CreatedID != userID {
		return fmt.Errorf("권한 없음: 자신이 등록한 상세 채널만 수정할 수 있습니다.")
	}

	detail := &ChannelDetail{
		ID:          detailID,
		ChannelName: req.ChannelName,
		ChannelID:   req.ChannelID,
	}

	err = s.store.UpdateChannelDetail(detail)
	if err != nil {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok {
			if mysqlErr.Number == ErrMySQLDuplicateEntry {
				// (중복 에러 처리)
			}
		}
		return err
	}
	return nil
}

// DeleteChannelDetail은 '권한' 확인 후 상세 채널을 삭제합니다.
func (s *Service) DeleteChannelDetail(detailID uint64, userID uint64, userRole string) error {
	originalDetail, err := s.store.GetChannelDetailByID(detailID)
	if err != nil {
		return fmt.Errorf("삭제할 상세 채널(ID: %d)을 찾을 수 없습니다.", detailID)
	}
	if userRole != "ADMIN" && originalDetail.CreatedID != userID {
		return fmt.Errorf("권한 없음: 자신이 등록한 상세 채널만 삭제할 수 있습니다.")
	}

	err = s.store.DeleteChannelDetail(detailID)
	if err != nil {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok {
			// (DBA 님) 'channel_group_mapping' FK 위배
			if mysqlErr.Number == ErrMySQLForeignKeyFail {
				return fmt.Errorf("삭제 실패: 이 상세 채널을 사용 중인 '채널 그룹 매핑'이 있습니다.")
			}
		}
		return err
	}
	return nil
}