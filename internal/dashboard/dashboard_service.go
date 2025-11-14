package dashboard

import (
	"log"

	// (주의) 다른 패키지(channel, notice, template)의 Store를 사용합니다.
	"harbinger/internal/channel"
	"harbinger/internal/notice"
	"harbinger/internal/template"

	"golang.org/x/sync/errgroup" // (여러 DB 조회를 병렬로 처리하기 위함)
)

// DashboardData는 대시보드 뷰(View)에 전달될 데이터 구조체입니다.
type DashboardData struct {
	ActiveNotices      []notice.NoticeSchedule // 활성 공지 목록
	TemplateCount      int                     // 템플릿 수
	ChannelGroupCount int                     // 채널 그룹 수
}

// Service는 대시보드 데이터 조회를 담당합니다.
// (여러 Store에 의존합니다)
type Service struct {
	noticeStore   *notice.Store
	templateStore *template.Store
	channelStore  *channel.Store
}

// NewService는 대시보드 서비스를 생성합니다.
func NewService(ns *notice.Store, ts *template.Store, cs *channel.Store) *Service {
	return &Service{
		noticeStore:   ns,
		templateStore: ts,
		channelStore:  cs,
	}
}

// GetDashboardData는 3가지 데이터를 DB에서 병렬로 조회하여 집계합니다.
func (s *Service) GetDashboardData(userID uint64, userRole string) (*DashboardData, error) {
	var data DashboardData
	var eg errgroup.Group 

	// 고루틴 1: 활성 공지 조회 (수정: 권한 인자 전달)
	eg.Go(func() error {
		notices, err := s.noticeStore.GetActiveNotices(userID, userRole)
		if err != nil {
			log.Printf("[ERROR] GetDashboardData: GetActiveNotices 실패: %v", err)
			return err
		}
		data.ActiveNotices = notices
		return nil
	})

	// 고루틴 2: 템플릿 수 조회
	eg.Go(func() error {
		count, err := s.templateStore.CountTemplates()
		if err != nil {
			log.Printf("[ERROR] GetDashboardData: CountTemplates 실패: %v", err)
			return err
		}
		data.TemplateCount = count
		return nil
	})

	// 고루틴 3: 채널 그룹 수 조회
	eg.Go(func() error {
		count, err := s.channelStore.CountChannelGroups()
		if err != nil {
			log.Printf("[ERROR] GetDashboardData: CountChannelGroups 실패: %v", err)
			return err
		}
		data.ChannelGroupCount = count
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return &data, nil 
}