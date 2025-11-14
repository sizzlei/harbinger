package scheduler

import (
	"log"
	"sync"

	"github.com/robfig/cron/v3"

	// (Slack 관련 import 모두 제거 - service가 담당)
	
	"harbinger/internal/notice" // (notice.Store와 notice.Service가 모두 필요)
)

// (SlackMessageWrapper, SlackMentionBlock 구조체 모두 제거)

// Scheduler
type Scheduler struct {
	cron *cron.Cron
	// (의존성)
	noticeStore   *notice.Store
	noticeService *notice.Service // (notice.Service 의존성)
}

// NewScheduler
func NewScheduler(ns *notice.Store, nSvc *notice.Service) *Scheduler {
	c := cron.New()
	return &Scheduler{
		cron:          c,
		noticeStore:   ns,
		noticeService: nSvc,
	}
}

// Start
func (s *Scheduler) Start() {
	log.Println("[INFO] -----------------------------------------")
	log.Println("[INFO] 🔔 Harbinger 스케줄러가 시작됩니다...")
	s.cron.AddFunc("@every 1m", s.checkAndSendNotices)
	s.cron.Start()
	log.Println("[INFO] -----------------------------------------")
}

// Stop
func (s *Scheduler) Stop() {
	log.Println("[INFO] Harbinger 스케줄러가 중지됩니다...")
	s.cron.Stop()
}

// checkAndSendNotices (수정됨)
func (s*Scheduler) checkAndSendNotices() {
	log.Println("[Scheduler] 1분마다 공지 대상을 확인합니다...")

	// 1. (DB) "지금" 발송해야 할 공지 목록 가져오기
	notices, err := s.noticeStore.GetNoticesToRunNow()
	if err != nil {
		log.Printf("[ERROR] [Scheduler] 공지 목록 조회 실패: %v", err)
		return
	}

	if len(notices) == 0 {
		log.Println("[Scheduler] 발송할 공지가 없습니다.")
		return
	}

	log.Printf("[Scheduler] %d 건의 공지 발송을 시작합니다.", len(notices))

	var wg sync.WaitGroup
	for _, ns := range notices {
		wg.Add(1)
		
		// (수정) 'notice.Schedule' -> 'notice.NoticeSchedule'
		go func(n notice.NoticeSchedule) {
			defer wg.Done()
			// (수정) 발송 로직을 'noticeService'에 위임
			if err := s.noticeService.SendScheduledNotice(&n); err != nil {
				// (SendScheduledNotice가 이미 상세 로그를 찍음)
				log.Printf("[ERROR] [Scheduler] 공지(ID: %d) 처리 중 에러 발생", n.ID)
			}
		}(ns)
	}
	wg.Wait()
	log.Printf("[Scheduler] %d 건의 공지 발송 작업이 완료되었습니다.", len(notices))
}

// (processNotice 함수 제거됨)