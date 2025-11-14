package notice

import (
	"encoding/json" 
	"fmt"
	"log"
	"strings" 
	"time"    

	"github.com/go-sql-driver/mysql" 
	"github.com/sizzlei/slack-notificator"
	"github.com/slack-go/slack"
	"golang.org/x/sync/errgroup" 

	"harbinger/internal/channel"
	"harbinger/internal/slackbot" 
	"harbinger/internal/template"
)

// (MySQL 'Duplicate entry' 에러 코드)
const (
	ErrMySQLDuplicateEntry = 1062
)

// (Slack 메시지 구조체 - 변경 없음)
type SlackMessageWrapper struct {
	Color  string        `json:"Color"`
	Blocks []interface{} `json:"blocks"`
}
type SlackMentionBlock struct {
	Type string `json:"type"`
	Text struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"text"`
}

// Service (변경 없음)
type Service struct {
	store         *Store
	channelStore  *channel.Store  
	templateStore *template.Store 
	slackbotStore *slackbot.Store 
}

// NewService (변경 없음)
func NewService(store *Store, cs *channel.Store, ts *template.Store, sbs *slackbot.Store) *Service {
	return &Service{
		store:         store,
		channelStore:  cs,
		templateStore: ts,
		slackbotStore: sbs, 
	}
}

// CreatePageData (변경 없음)
type CreatePageData struct {
	ChannelGroups []channel.ChannelGroup
	Templates     []template.Template
	Slackbots     []slackbot.SlackbotConfig 
}

// GetCreatePageData (변경 없음)
func (s *Service) GetCreatePageData() (*CreatePageData, error) {
	var data CreatePageData
	var eg errgroup.Group

	eg.Go(func() error {
		groups, err := s.channelStore.GetAllChannelGroups()
		if err != nil { return err }
		data.ChannelGroups = groups
		return nil
	})
	eg.Go(func() error {
		templates, err := s.templateStore.GetAllTemplates()
		if err != nil { return err }
		data.Templates = templates
		return nil
	})
	eg.Go(func() error {
		bots, err := s.slackbotStore.GetAllSlackbots()
		if err != nil { return err }
		data.Slackbots = bots
		return nil
	})

	if err := eg.Wait(); err != nil {
		log.Printf("[ERROR] GetCreatePageData 조회 실패: %v", err)
		return nil, err
	}
	return &data, nil
}

// (NoticeContentForm, CreateNoticeRequest, parseFormToModel, CreateNotice, GetNoticeScheduleByID, UpdateNotice, DeleteNotice 함수는 변경 없음)
type NoticeContentForm struct {
	ContentTitle string `form:"content_title"`
	ContentBody  string `form:"content_body"`
	ContentRefer string `form:"content_refer"`
}
type CreateNoticeRequest struct {
	NoticeTitle    string `form:"notice_title"`
	TemplateID     uint64 `form:"template_id"`
	MessageType    string `form:"message_type"`
	ChannelGroupID uint64 `form:"channel_group_id"`
	NoticeStartDe  string `form:"notice_start_de"` 
	NoticeEndDe    string `form:"notice_end_de"`   
	NoticeTime     string `form:"notice_time"`     
	NoticeInterval string `form:"notice_interval"` 
	HereYn         bool   `form:"here_yn"`
	ChannelYn      bool   `form:"channel_yn"`
	SlackbotID     uint64 `form:"slackbot_id"`
	NoticeContentForm
}
func (s *Service) parseFormToModel(req CreateNoticeRequest) (*NoticeSchedule, error) {
	contentMap := map[string]string{
		"title":   req.ContentTitle,
		"content": req.ContentBody,
		"refer":   req.ContentRefer,
	}
	contentJSON, err := json.Marshal(contentMap)
	if err != nil {
		return nil, fmt.Errorf("컨텐츠 JSON 생성 실패")
	}
	startDate, err1 := time.Parse("2006-01-02", req.NoticeStartDe)
	endDate, err2 := time.Parse("2006-01-02", req.NoticeEndDe)
	noticeTime := req.NoticeTime + ":00" 
	if err1 != nil || err2 != nil {
		return nil, fmt.Errorf("날짜 형식이 잘못되었습니다 (YYYY-MM-DD).")
	}
	ns := &NoticeSchedule{
		NoticeTitle:      req.NoticeTitle,
		TemplateID:       req.TemplateID,
		MessageType:      req.MessageType, // (신규)
		ChannelGroupID:   req.ChannelGroupID,
		NoticeStartDe:    startDate,
		NoticeEndDe:      endDate,
		NoticeTime:       noticeTime,
		NoticeInterval:   req.NoticeInterval,
		HereYn:           req.HereYn,
		ChannelYn:        req.ChannelYn,
		NoticeContents:   string(contentJSON),
		SlackbotID:       req.SlackbotID,
	}
	return ns, nil
}
func (s *Service) CreateNotice(req CreateNoticeRequest, createdID uint64) error {
	ns, err := s.parseFormToModel(req)
	if err != nil { return err }
	ns.CreatedID = createdID 
	err = s.store.CreateNoticeSchedule(ns)
	if err != nil {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok {
			if mysqlErr.Number == ErrMySQLDuplicateEntry && strings.Contains(mysqlErr.Message, "udx_notice_schedules_01") {
				return fmt.Errorf("이미 존재하는 공지 제목입니다: %s", req.NoticeTitle)
			}
		}
		log.Printf("[ERROR] CreateNotice 서비스 에러: %v", err)
		return err
	}
	return nil
}
func (s *Service) GetNoticeScheduleByID(id uint64) (*NoticeSchedule, error) {
	return s.store.GetNoticeScheduleByID(id)
}
func (s *Service) UpdateNotice(req CreateNoticeRequest, noticeID uint64, userID uint64, userRole string) error {
	originalNotice, err := s.store.GetNoticeScheduleByID(noticeID)
	if err != nil {
		return fmt.Errorf("수정할 공지(ID: %d)를 찾을 수 없습니다.", noticeID)
	}
	if userRole != "ADMIN" && originalNotice.CreatedID != userID {
		return fmt.Errorf("권한 없음: 자신이 작성한 공지만 수정할 수 있습니다.")
	}
	ns, err := s.parseFormToModel(req)
	if err != nil { return err }
	ns.ID = noticeID 
	err = s.store.UpdateNoticeSchedule(ns)
	if err != nil {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok {
			if mysqlErr.Number == ErrMySQLDuplicateEntry && strings.Contains(mysqlErr.Message, "udx_notice_schedules_01") {
				return fmt.Errorf("이미 존재하는 공지 제목입니다: %s", req.NoticeTitle)
			}
		}
		log.Printf("[ERROR] UpdateNotice 서비스 에러: %v", err)
		return err
	}
	return nil
}
func (s *Service) DeleteNotice(noticeID uint64, userID uint64, userRole string) error {
	originalNotice, err := s.store.GetNoticeScheduleByID(noticeID)
	if err != nil {
		return fmt.Errorf("삭제할 공지(ID: %d)를 찾을 수 없습니다.", noticeID)
	}
	if userRole != "ADMIN" && originalNotice.CreatedID != userID {
		return fmt.Errorf("권한 없음: 자신이 작성한 공지만 삭제할 수 있습니다.")
	}
	return s.store.DeleteNoticeSchedule(noticeID)
}

// getAssembledMessage: 공지 ID를 받아 최종 멘션과 템플릿(Attachment)을 조립합니다.
// (반환 값 변경: UserEnteredTitle 반환)
func (s *Service) getAssembledMessage(noticeID uint64) (string, string, slack.Attachment, error) {
	var attachment slack.Attachment 

	ns, err := s.store.GetNoticeScheduleByID(noticeID)
	if err != nil {
		return "", "", attachment, fmt.Errorf("공지(ID: %d) 조회 실패: %v", noticeID, err)
	}
	template, err := s.templateStore.GetTemplateByID(ns.TemplateID)
	if err != nil {
		return "", "", attachment, fmt.Errorf("템플릿(ID: %d) 조회 실패: %v", ns.TemplateID, err)
	}

	// 3. (로직) 템플릿(<title>)과 내용(JSON) 매핑
	var contentsMap map[string]string
	if err := json.Unmarshal([]byte(ns.NoticeContents), &contentsMap); err != nil {
		return "", "", attachment, fmt.Errorf("공지(ID: %d) 'NoticeContents' JSON 파싱 실패: %v", noticeID, err)
	}
    
    // (신규) 1. <title>에 들어갈 최종 제목 (알림창 텍스트용)
    // 이 필드는 'contentTitle' 키를 통해 가져옵니다.
    contentTitle := contentsMap["title"] 

	// 5. (로직) 멘션(@here, @channel) 텍스트 준비
	mentionText := ""
	if ns.HereYn {
		mentionText += "<!here> \n"
	}
	if ns.ChannelYn {
		mentionText += "<!channel> \n"
	}

	// 4. (신규) Raw 텍스트 (title, content, refer)를 줄바꿈으로 연결
	rawBody := fmt.Sprintf("%s\n%s", contentsMap["content"], contentsMap["refer"])
	
	// 5. (신규) Plain Message일 경우, Attachment 객체의 Text 필드에 rawBody를 담아 반환
	if ns.MessageType == "PLAIN" {
		attachment.Text = rawBody
		return mentionText, contentTitle, attachment, nil
	}

	finalJsonString := template.TemplateContents
	for key, value := range contentsMap {
		// 타이틀은 본문 제외
		if key != "title" {
			placeholder := fmt.Sprintf("<%s>", key)

			// 1. Windows 줄바꿈(\r\n) 또는 Mac 구형(\r)에서 \r을 제거
			value = strings.ReplaceAll(value, "\r\n", "\n")
			value = strings.ReplaceAll(value, "\r", "\n")

			// 2. 'value' (이제 \n만 포함)를 JSON 문자열로 이스케이프
			escapedValue, err := json.Marshal(value)
			if err != nil {
				log.Printf("[ERROR] [Scheduler] JSON 값 이스케이프 실패 (Key: %s): %v", key, err)
				continue
			}

			// 3. Marshal이 추가한 앞뒤 큰따옴표(") 제거
			valueString := string(escapedValue)
			if len(valueString) >= 2 {
				valueString = valueString[1 : len(valueString)-1]
			}
			
			finalJsonString = strings.ReplaceAll(finalJsonString, placeholder, valueString)
		}
	}

	// 4. (로직) 'sizzlei/slack-notificator'의 'CreateAttachement' 사용
	attachment, err = slacknotificator.CreateAttachement(finalJsonString)
	if err != nil {
		return "", "", attachment, fmt.Errorf("공지(ID: %d)의 최종 템플릿 JSON 파싱/변환 실패: %v", noticeID, err)
	}

	// 6. (수정) 3가지 값을 반환: (멘션, 유저 입력 제목, Attachment)
	return mentionText, contentTitle, attachment, nil
}


// --- (SendScheduledNotice - 수정 6) ---
// (수정) SendScheduledNotice 로직 수정
func (s *Service) SendScheduledNotice(ns *NoticeSchedule) error {
	log.Printf("[Scheduler] 공지 처리 시작 (ID: %d, 제목: %s)", ns.ID, ns.NoticeTitle)
	var botToken string
	var slackChannelIDs []string
	var eg errgroup.Group

	// 1. (DB) 봇 토큰 조회
	eg.Go(func() error {
		token, err := s.slackbotStore.GetBotTokenByID(ns.SlackbotID) 
		if err != nil {
			return fmt.Errorf("봇 토큰(ID: %d) 조회 실패: %v", ns.SlackbotID, err)
		}
		botToken = token
		return nil
	})
	// 2. (DB) 채널 그룹 ID 목록 조회
	eg.Go(func() error {
		ids, err := s.channelStore.GetSlackIDsByGroupID(ns.ChannelGroupID)
		if err != nil {
			return fmt.Errorf("채널 목록(GroupID: %d) 조회 실패: %v", ns.ChannelGroupID, err)
		}
		slackChannelIDs = ids
		return nil
	})
	if err := eg.Wait(); err != nil {
		log.Printf("[ERROR] [Scheduler] 공지(ID: %d) 데이터 준비 실패: %v", ns.ID, err)
		return err
	}

	// 3. (로직) 메시지 조립
	// (수정) UserEnteredTitle 값 받기
	mentionText, contentTitle, attachment, err := s.getAssembledMessage(ns.ID)
	if err != nil {
		log.Printf("[ERROR] [Scheduler] 공지(ID: %d) 메시지 조립 실패: %v", ns.ID, err)
		return err
	}

	// 4. (API) 'slack-notificator'로 발송
	notificationText := mentionText + contentTitle
	api := slacknotificator.GetClient(botToken)
	for _, channelID := range slackChannelIDs {
		fmt.Println(ns.MessageType)
		if ns.MessageType == "PLAIN" {
			fmt.Println(notificationText + "\n\n" + strings.TrimSpace(attachment.Text))
			err := api.SetChannel(channelID).SendMessage(notificationText + "\n\n" + strings.TrimSpace(attachment.Text))
			if err != nil {
				log.Printf("[ERROR] [Scheduler] 공지(ID: %d) -> 채널(%s) Plain 발송 실패: %v", ns.ID, channelID, err)
			} else {
				log.Printf("[SUCCESS] [Scheduler] 공지(ID: %d) -> 채널(%s) Plain 발송 성공", ns.ID, channelID)
			}
		} else {
			// (수정) notificationText 전달
			err := api.SetChannel(channelID).SendAttachment(notificationText, attachment) 
			if err != nil {
				log.Printf("[ERROR] [Scheduler] 공지(ID: %d) -> 채널(%s) 발송 실패: %v", ns.ID, channelID, err)
			} else {
				log.Printf("[SUCCESS] [Scheduler] 공지(ID: %d) -> 채널(%s) 발송 성공", ns.ID, channelID)
			}
		}
		
	}
	return nil
}

// TestSendNotice: '테스트 발송' 핸들러가 호출할 함수
func (s *Service) TestSendNotice(noticeID uint64, userEmail string) error {
	log.Printf("[TestSend] 테스트 발송 시작 (NoticeID: %d, User: %s)", noticeID, userEmail)
	ns, err := s.store.GetNoticeScheduleByID(noticeID)
	if err != nil {
		return fmt.Errorf("공지(ID: %d) 조회 실패: %v", noticeID, err)
	}

	// 2. (DB) 봇 토큰 조회
	botToken, err := s.slackbotStore.GetBotTokenByID(ns.SlackbotID)
	if err != nil {
		return fmt.Errorf("봇 토큰(ID: %d) 조회 실패: %v", ns.SlackbotID, err)
	}

	// 3. (로직) 메시지 조립
	// (수정) UserEnteredTitle 값 받기
	mentionText, contentTitle, attachment, err := s.getAssembledMessage(noticeID)
	if err != nil {
		return fmt.Errorf("메시지 조립 실패: %v", err)
	}

	// 4. (API) 'slack-notificator'로 DM 발송
	api := slacknotificator.GetClient(botToken)
	memberID, err := api.GetMemberId(userEmail)
	if err != nil {
		return fmt.Errorf("Slack 사용자(%s) ID 조회 실패: %v", userEmail, err)
	}
	if err := api.CreateDMChannel(*memberID); err != nil {
		return fmt.Errorf("DM 채널(%s) 생성 실패: %v", userEmail, err)
	}
	
	// (수정) 알림창 메시지 결합: [멘션] + [유저 입력 제목]
	notificationText := mentionText + contentTitle

	if ns.MessageType == "PLAIN" {
		err = api.SendMessage(notificationText + "\n\n" + strings.TrimSpace(attachment.Text))
		if err != nil {
			log.Printf("[ERROR] [TestSend] 공지(ID: %d) -> DM(%s) Plain 발송 실패: %v", noticeID, userEmail, err)
			return err
		}
	} else {
		err = api.SendAttachment(notificationText, attachment)
		if err != nil {
			log.Printf("[ERROR] [TestSend] 공지(ID: %d) -> DM(%s) 발송 실패: %v", noticeID, userEmail, err)
			return err
		}
	}

	log.Printf("[SUCCESS] [TestSend] 공지(ID: %d) -> DM(%s) 발송 성공", noticeID, userEmail)
	return nil
}