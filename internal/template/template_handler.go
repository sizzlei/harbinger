package template

import (
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session" // (플래시 메시지용)
	log "github.com/sirupsen/logrus"
)

// TemplateHandler는 템플릿 관련 핸들러입니다.
type TemplateHandler struct {
	service *Service
	store   *session.Store
}

// NewTemplateHandler는 새 핸들러를 생성합니다.
func NewTemplateHandler(service *Service, store *session.Store) *TemplateHandler {
	return &TemplateHandler{
		service: service,
		store:   store,
	}
}

// HandleShowTemplatePage는 'GET /templates' 요청을 처리합니다.
func (h *TemplateHandler) HandleShowTemplatePage(c *fiber.Ctx) error {
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

	// 2. 서비스 호출 (템플릿 목록 조회)
	templates, err := h.service.GetAllTemplates()
	if err != nil {
		log.Errorf("템플릿 페이지 데이터 조회 실패: %v", err)
		return c.Status(500).SendString("데이터 조회 중 오류 발생")
	}

	// 3. (수정) Locals에서 UserRole 가져오기
	userEmail := c.Locals("user_email").(string)
	userRole := c.Locals("user_role").(string)

	// 4. 'templates.html' 뷰(View)에 데이터 전달
	return c.Render("templates", fiber.Map{
		"Title":        "Harbinger | 템플릿 관리",
		"UserEmail":    userEmail,
		"UserRole":     userRole, // (layout.html이 사용할 수 있도록 역할 전달)
		"Templates":    templates,
		"FlashSuccess": flashSuccess,
		"FlashError":   flashError,
	}, "layout")
}

// HandleCreateTemplate는 'POST /templates' 요청을 처리합니다.
func (h *TemplateHandler) HandleCreateTemplate(c *fiber.Ctx) error {
	// 1. 폼 데이터 파싱
	type templateForm struct {
		TemplateName     string `form:"template_name"`
		TemplateContents string `form:"template_contents"`
	}
	form := new(templateForm)
	if err := c.BodyParser(form); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("템플릿 폼 입력이 잘못되었습니다.")
	}

	createdID := c.Locals("user_id").(uint64)
	sess, _ := h.store.Get(c)

	// 2. 서비스 호출
	err := h.service.CreateTemplate(CreateTemplateRequest{
		TemplateName:     form.TemplateName,
		TemplateContents: form.TemplateContents,
	}, createdID)

	if err != nil {
		log.Errorf("템플릿 생성 실패: %v", err)
		sess.Set("flash_error", "템플릿 생성 실패: "+err.Error())
	} else {
		sess.Set("flash_success", "템플릿이 성공적으로 생성되었습니다.")
	}
	sess.Save()

	return c.Redirect("/templates")
}

// HandleShowEditTemplatePage는 'GET /templates/edit/:id' 요청을 처리합니다.
func (h *TemplateHandler) HandleShowEditTemplatePage(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(400).SendString("유효하지 않은 ID입니다.")
	}

	// 1. 서비스 호출 (템플릿 1개 조회)
	template, err := h.service.GetTemplateByID(uint64(id))
	if err != nil {
		log.Errorf("템플릿 조회 실패(ID: %d): %v", id, err)
		// (TODO: 404 페이지 처리)
		return c.Status(404).SendString("템플릿을 찾을 수 없습니다.")
	}

	// 2. (수정) Locals에서 UserRole 가져오기
	userEmail := c.Locals("user_email").(string)
	userRole := c.Locals("user_role").(string)

	// 3. 'templates_edit.html' 뷰(View)에 데이터 전달
	return c.Render("templates_edit", fiber.Map{
		"Title":     fmt.Sprintf("Harbinger | 템플릿 수정 (ID: %d)", id),
		"UserEmail": userEmail,
		"UserRole":  userRole, // (layout.html이 사용할 수 있도록 역할 전달)
		"Template":  template,
	}, "layout")
}

// HandleUpdateTemplate는 'POST /templates/edit/:id' 요청을 처리합니다.
func (h *TemplateHandler) HandleUpdateTemplate(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(400).SendString("유효하지 않은 ID입니다.")
	}

	// 1. 폼 데이터 파싱
	type templateForm struct {
		TemplateName     string `form:"template_name"`
		TemplateContents string `form:"template_contents"`
	}
	form := new(templateForm)
	if err := c.BodyParser(form); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("템플릿 폼 입력이 잘못되었습니다.")
	}

	// 2. (권한) 미들웨어에서 'user_id'와 'user_role' 가져오기
	userID := c.Locals("user_id").(uint64)
	userRole := c.Locals("user_role").(string)
	sess, _ := h.store.Get(c)

	// 3. 서비스 호출 (권한 인자 전달)
	err = h.service.UpdateTemplate(UpdateTemplateRequest{
		ID:               uint64(id),
		TemplateName:     form.TemplateName,
		TemplateContents: form.TemplateContents,
	}, userID, userRole)

	if err != nil {
		log.Errorf("템플릿 수정 실패: %v", err)
		sess.Set("flash_error", "템플릿 수정 실패: "+err.Error())
	} else {
		sess.Set("flash_success", "템플릿(ID: "+strconv.Itoa(id)+")이 성공적으로 수정되었습니다.")
	}
	sess.Save()
	
	return c.Redirect("/templates")
}

// HandleDeleteTemplate는 'POST /templates/delete/:id' 요청을 처리합니다.
func (h *TemplateHandler) HandleDeleteTemplate(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(400).SendString("유효하지 않은 ID입니다.")
	}

	// 1. (권한) 미들웨어에서 'user_id'와 'user_role' 가져오기
	userID := c.Locals("user_id").(uint64)
	userRole := c.Locals("user_role").(string)
	sess, _ := h.store.Get(c)

	// 2. 서비스 호출 (권한 인자 전달)
	err = h.service.DeleteTemplate(uint64(id), userID, userRole)

	if err != nil {
		log.Errorf("템플릿 삭제 실패: %v", err)
		sess.Set("flash_error", "템플릿 삭제 실패: "+err.Error())
	} else {
		sess.Set("flash_success", "템플릿(ID: "+strconv.Itoa(id)+")이 성공적으로 삭제되었습니다.")
	}
	sess.Save()

	return c.Redirect("/templates")
}