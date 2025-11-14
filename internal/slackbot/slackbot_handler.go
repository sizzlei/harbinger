package slackbot

import (
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session" // (플래시 메시지용)
	log "github.com/sirupsen/logrus"
)

// SlackbotHandler는 봇 관련 핸들러입니다.
type SlackbotHandler struct {
	service *Service
	store   *session.Store
}

// NewSlackbotHandler는 새 핸들러를 생성합니다.
func NewSlackbotHandler(service *Service, store *session.Store) *SlackbotHandler {
	return &SlackbotHandler{
		service: service,
		store:   store,
	}
}

// HandleShowBotPage는 'GET /bots' 요청을 처리합니다.
func (h *SlackbotHandler) HandleShowBotPage(c *fiber.Ctx) error {
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

	// 2. 서비스 호출 (봇 목록 조회)
	bots, err := h.service.GetAllSlackbots()
	if err != nil {
		log.Errorf("봇 페이지 데이터 조회 실패: %v", err)
		return c.Status(500).SendString("데이터 조회 중 오류 발생")
	}

	// 3. Locals에서 UserRole 가져오기
	userEmail := c.Locals("user_email").(string)
	userRole := c.Locals("user_role").(string)

	// 4. 'bots.html' 뷰(View)에 데이터 전달
	return c.Render("bots", fiber.Map{
		"Title":        "Harbinger | Slack 봇 관리",
		"UserEmail":    userEmail,
		"UserRole":     userRole, // (layout.html이 사용할 수 있도록 역할 전달)
		"Bots":         bots,
		"FlashSuccess": flashSuccess,
		"FlashError":   flashError,
	}, "layout")
}

// HandleCreateBot는 'POST /bots' 요청을 처리합니다.
func (h *SlackbotHandler) HandleCreateBot(c *fiber.Ctx) error {
	// 1. 폼 데이터 파싱
	type botForm struct {
		BotName  string `form:"bot_name"`
		BotToken string `form:"bot_token"`
	}
	form := new(botForm)
	if err := c.BodyParser(form); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("봇 폼 입력이 잘못되었습니다.")
	}

	createdID := c.Locals("user_id").(uint64)
	sess, _ := h.store.Get(c)

	// 2. 서비스 호출
	err := h.service.CreateSlackbot(CreateBotRequest{
		BotName:  form.BotName,
		BotToken: form.BotToken,
	}, createdID)

	if err != nil {
		log.Errorf("봇 생성 실패: %v", err)
		sess.Set("flash_error", "봇 생성 실패: "+err.Error())
	} else {
		sess.Set("flash_success", "Slack 봇이 성공적으로 등록되었습니다.")
	}
	sess.Save()

	return c.Redirect("/bots")
}

// HandleShowEditBotPage는 'GET /bots/edit/:id' 요청을 처리합니다.
func (h *SlackbotHandler) HandleShowEditBotPage(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(400).SendString("유효하지 않은 ID입니다.")
	}
	
	// (신규) 기본 봇(ID=1)은 수정 페이지 접근 차단 (UI 보호)
	if id == 1 {
		sess, _ := h.store.Get(c)
		sess.Set("flash_error", "기본 봇(ID: 1)은 수정할 수 없습니다.")
		sess.Save()
		return c.Redirect("/bots")
	}

	// 1. 서비스 호출 (봇 1개 조회)
	bot, err := h.service.GetSlackbotByID(uint64(id))
	if err != nil {
		log.Errorf("봇 조회 실패(ID: %d): %v", id, err)
		return c.Status(404).SendString("봇을 찾을 수 없습니다.")
	}

	// 2. Locals에서 UserRole 가져오기
	userEmail := c.Locals("user_email").(string)
	userRole := c.Locals("user_role").(string)

	// 3. 'bots_edit.html' 뷰(View)에 데이터 전달
	return c.Render("bots_edit", fiber.Map{
		"Title":     fmt.Sprintf("Harbinger | 봇 수정 (ID: %d)", id),
		"UserEmail": userEmail,
		"UserRole":  userRole, // (layout.html이 사용할 수 있도록 역할 전달)
		"Bot":       bot,
	}, "layout")
}

// HandleUpdateBot는 'POST /bots/edit/:id' 요청을 처리합니다.
func (h *SlackbotHandler) HandleUpdateBot(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(400).SendString("유효하지 않은 ID입니다.")
	}

	// 1. 폼 데이터 파싱
	type botForm struct {
		BotName  string `form:"bot_name"`
		BotToken string `form:"bot_token"`
	}
	form := new(botForm)
	if err := c.BodyParser(form); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("봇 폼 입력이 잘못되었습니다.")
	}

	// 2. (수정) 권한 정보 가져오기
	userID := c.Locals("user_id").(uint64)
	userRole := c.Locals("user_role").(string)
	sess, _ := h.store.Get(c)

	// 3. (수정) 서비스 호출 (권한 인자 전달)
	err = h.service.UpdateSlackbot(UpdateBotRequest{
		ID:       uint64(id),
		BotName:  form.BotName,
		BotToken: form.BotToken,
	}, userID, userRole)

	if err != nil {
		log.Errorf("봇 수정 실패: %v", err)
		sess.Set("flash_error", "봇 수정 실패: "+err.Error())
	} else {
		sess.Set("flash_success", "봇(ID: "+strconv.Itoa(id)+")이 성공적으로 수정되었습니다.")
	}
	sess.Save()

	return c.Redirect("/bots")
}

// HandleDeleteBot는 'POST /bots/delete/:id' 요청을 처리합니다.
func (h *SlackbotHandler) HandleDeleteBot(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(400).SendString("유효하지 않은 ID입니다.")
	}

	// 1. (수정) 권한 정보 가져오기
	userID := c.Locals("user_id").(uint64)
	userRole := c.Locals("user_role").(string)
	sess, _ := h.store.Get(c)

	// 2. (수정) 서비스 호출 (권한 인자 전달)
	err = h.service.DeleteSlackbot(uint64(id), userID, userRole)

	if err != nil {
		log.Errorf("봇 삭제 실패: %v", err)
		sess.Set("flash_error", "봇 삭제 실패: "+err.Error())
	} else {
		sess.Set("flash_success", "봇(ID: "+strconv.Itoa(id)+")이 성공적으로 삭제되었습니다.")
	}
	sess.Save()

	return c.Redirect("/bots")
}