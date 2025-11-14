package notice

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	log "github.com/sirupsen/logrus" // (logrus 표준 사용)
)

// NoticeHandler는 공지 관련 핸들러입니다.
type NoticeHandler struct {
	service *Service
	store   *session.Store
}

// NewNoticeHandler는 새 핸들러를 생성합니다.
func NewNoticeHandler(service *Service, store *session.Store) *NoticeHandler {
	return &NoticeHandler{
		service: service,
		store:   store,
	}
}

// HandleShowNoticePage는 'GET /notices' 요청을 처리합니다.
func (h *NoticeHandler) HandleShowNoticePage(c *fiber.Ctx) error {
	sess, _ := h.store.Get(c)

	// 1. 플래시 메시지 읽기
	flashSuccess := sess.Get("flash_success")
	flashError := sess.Get("flash_error")
	if flashSuccess != nil {
		sess.Delete("flash_success")
	}
	if flashError != nil {
		sess.Delete("flash_error")
	}
	sess.Save()

	// 2. 서비스 호출 (폼 드롭다운에 필요한 데이터)
	formData, err := h.service.GetCreatePageData()
	if err != nil {
		log.Errorf("공지 페이지 폼 데이터 조회 실패: %v", err)
		return c.Status(500).SendString("데이터 조회 중 오류 발생 (폼)")
	}

	// 3. (수정) Locals에서 권한 정보 가져오기
	userEmail := c.Locals("user_email").(string)
	userRole := c.Locals("user_role").(string)
	userID := c.Locals("user_id").(uint64)

	// 4. (수정) 공지 목록 데이터 (권한 인자 전달)
	notices, err := h.service.store.GetActiveNotices(userID, userRole)
	if err != nil {
		log.Errorf("공지 페이지 목록 데이터 조회 실패: %v", err)
		return c.Status(500).SendString("데이터 조회 중 오류 발생 (목록)")
	}

	// 5. 'notices.html' 뷰(View)에 데이터 전달
	return c.Render("notices", fiber.Map{
		"Title":         "Harbinger | 공지 관리",
		"UserEmail":     userEmail,
		"UserRole":      userRole,
		"FormData":      formData, 
		"Notices":       notices,
		"FlashSuccess":  flashSuccess,
		"FlashError":    flashError,
	}, "layout")
}

// HandleCreateNotice는 'POST /notices' 요청을 처리합니다.
func (h *NoticeHandler) HandleCreateNotice(c *fiber.Ctx) error {
	// 1. 폼 데이터 파싱
	var req CreateNoticeRequest
	if err := c.BodyParser(&req); err != nil {
		log.Warnf("공지 생성 폼 파싱 실패: %v", err)
		return c.Status(fiber.StatusBadRequest).SendString("공지 폼 입력이 잘못되었습니다.")
	}

	createdID := c.Locals("user_id").(uint64)
	sess, _ := h.store.Get(c)

	// 2. 서비스 호출
	err := h.service.CreateNotice(req, createdID)

	if err != nil {
		log.Errorf("공지 생성 실패: %v", err)
		sess.Set("flash_error", "공지 생성 실패: "+err.Error())
	} else {
		sess.Set("flash_success", "공지 스케줄이 성공적으로 등록되었습니다.")
	}
	sess.Save()

	return c.Redirect("/notices")
}

// HandleShowEditNoticePage는 'GET /notices/edit/:id' 요청을 처리합니다.
func (h *NoticeHandler) HandleShowEditNoticePage(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(400).SendString("유효하지 않은 ID입니다.")
	}

	// 1. 세션 및 플래시 메시지 읽기
	sess, _ := h.store.Get(c)
	flashSuccess := sess.Get("flash_success")
	flashError := sess.Get("flash_error")
	if flashSuccess != nil {
		sess.Delete("flash_success")
	}
	if flashError != nil {
		sess.Delete("flash_error")
	}
	sess.Save()

	// 2. 폼 드롭다운에 필요한 데이터 (채널 그룹, 템플릿, 봇)
	formData, err := h.service.GetCreatePageData()
	if err != nil {
		return c.Status(500).SendString("폼 데이터 조회 실패")
	}

	// 3. 수정할 공지 스케줄 원본 데이터
	notice, err := h.service.GetNoticeScheduleByID(uint64(id))
	if err != nil {
		return c.Status(404).SendString("공지 스케줄을 찾을 수 없습니다.")
	}

	// 4. 원본 'notice_contents' (JSON)를 맵(map)으로 파싱
	var contentsMap map[string]string
	if err := json.Unmarshal([]byte(notice.NoticeContents), &contentsMap); err != nil {
		log.Warnf("Notice(ID: %d) 템플릿 JSON 파싱 실패: %v", notice.ID, err)
		contentsMap = make(map[string]string)
	}

	// 5. Locals에서 UserRole 가져오기
	userEmail := c.Locals("user_email").(string)
	userRole := c.Locals("user_role").(string)

	// 6. 'notices_edit.html' 뷰(View)에 모든 데이터 전달
	return c.Render("notices_edit", fiber.Map{
		"Title":        fmt.Sprintf("Harbinger | 공지 수정 (ID: %d)", id),
		"UserEmail":    userEmail,
		"UserRole":     userRole,
		"FormData":     formData,    
		"Notice":       notice,      
		"ContentsMap":  contentsMap, 
		"FlashSuccess": flashSuccess,
		"FlashError":   flashError,
	}, "layout")
}

// HandleUpdateNotice는 'POST /notices/edit/:id' 요청을 처리합니다.
func (h *NoticeHandler) HandleUpdateNotice(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(400).SendString("유효하지 않은 ID입니다.")
	}

	// 1. 폼 데이터 파싱
	var req CreateNoticeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("공지 폼 입력이 잘못되었습니다.")
	}

	// 2. (권한) 미들웨어에서 'user_id'와 'user_role' 가져오기
	userID := c.Locals("user_id").(uint64)
	userRole := c.Locals("user_role").(string)
	sess, _ := h.store.Get(c)

	// 3. 서비스 호출 (권한 검사 포함)
	err = h.service.UpdateNotice(req, uint64(id), userID, userRole)

	if err != nil {
		log.Errorf("공지 수정 실패: %v", err)
		sess.Set("flash_error", "공지 수정 실패: "+err.Error())
	} else {
		sess.Set("flash_success", "공지 스케줄(ID: "+strconv.Itoa(id)+")이 성공적으로 수정되었습니다.")
	}
	sess.Save()

	return c.Redirect(fmt.Sprintf("/notices/edit/%d", id))
}

// HandleDeleteNotice는 'POST /notices/delete/:id' 요청을 처리합니다.
func (h *NoticeHandler) HandleDeleteNotice(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(400).SendString("유효하지 않은 ID입니다.")
	}

	// 1. (권한) 미들웨어에서 'user_id'와 'user_role' 가져오기
	userID := c.Locals("user_id").(uint64)
	userRole := c.Locals("user_role").(string)
	sess, _ := h.store.Get(c)

	// 2. 서비스 호출 (권한 검사 포함)
	err = h.service.DeleteNotice(uint64(id), userID, userRole)

	if err != nil {
		log.Errorf("공지 삭제 실패: %v", err)
		sess.Set("flash_error", "공지 삭제 실패: "+err.Error())
	} else {
		sess.Set("flash_success", "공지 스케줄(ID: "+strconv.Itoa(id)+")이 성공적으로 삭제되었습니다.")
	}
	sess.Save()

	return c.Redirect("/notices")
}

// HandleTestSendNotice는 'POST /notices/test/:id' 요청을 처리합니다.
func (h *NoticeHandler) HandleTestSendNotice(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(400).SendString("유효하지 않은 ID입니다.")
	}

	// 1. (권한) 미들웨어에서 'user_email' 가져오기 (DM 대상)
	userEmail := c.Locals("user_email").(string)
	sess, _ := h.store.Get(c)

	// 2. 서비스 호출 (테스트 발송)
	err = h.service.TestSendNotice(uint64(id), userEmail)

	if err != nil {
		log.Errorf("테스트 발송 실패: %v", err)
		sess.Set("flash_error", "테스트 발송 실패: "+err.Error())
	} else {
		sess.Set("flash_success", "테스트 발송 성공: "+userEmail+"님에게 DM을 발송했습니다.")
	}
	sess.Save()

	// 3. 원래 있던 페이지로 리다이렉트
	referer := c.Get("Referer")
	if referer != "" {
		return c.Redirect(referer)
	}
	return c.Redirect("/notices") // (기본값)
}