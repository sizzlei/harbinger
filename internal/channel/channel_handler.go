package channel

import (
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session" // (플래시 메시지용)
	log "github.com/sirupsen/logrus"                 // (logrus 표준 사용)
)

// ChannelHandler는 채널 관련 핸들러입니다.
type ChannelHandler struct {
	service *Service
	store   *session.Store
}

// NewChannelHandler는 새 핸들러를 생성합니다.
func NewChannelHandler(service *Service, store *session.Store) *ChannelHandler {
	return &ChannelHandler{
		service: service,
		store:   store,
	}
}

// HandleShowChannelPage는 'GET /channels' 요청을 처리합니다.
func (h *ChannelHandler) HandleShowChannelPage(c *fiber.Ctx) error {
	sess, err := h.store.Get(c)
	if err != nil {
		log.Errorf("HandleShowChannelPage: 세션 가져오기 실패: %v", err)
		return c.Redirect("/auth/login")
	}

	// 1. 플래시 메시지 읽기
	flashSuccess := sess.Get("flash_success")
	flashError := sess.Get("flash_error")
	if flashSuccess != nil {
		sess.Delete("flash_success")
	}
	if flashError != nil {
		sess.Delete("flash_error")
	}
	sess.Save() // (삭제 저장)

	// 2. 매핑을 위해 현재 선택된 그룹 ID 확인
	selectedGroupID, _ := strconv.ParseUint(c.Query("group_id"), 10, 64)

	// 3. 서비스 호출
	data, err := h.service.GetChannelListPageData(selectedGroupID)
	if err != nil {
		log.Errorf("채널 페이지 데이터 조회 실패: %v", err)
		return c.Status(500).SendString("데이터 조회 중 오류 발생")
	}

	// 4. Locals에서 UserRole 가져오기
	userEmail := c.Locals("user_email").(string)
	userRole := c.Locals("user_role").(string)

	// 5. 뷰(View)에 모든 데이터 전달
	return c.Render("channels", fiber.Map{
		"Title":        "Harbinger | 채널 관리",
		"UserEmail":    userEmail,
		"UserRole":     userRole, // (layout.html이 사용할 수 있도록 역할 전달)
		"Data":         data,
		"FlashSuccess": flashSuccess, // (성공 메시지 전달)
		"FlashError":   flashError,   // (에러 메시지 전달)
	}, "layout")
}

// HandleCreateChannelGroup은 'POST /channels/groups' 요청을 처리합니다. (모달 생성)
func (h *ChannelHandler) HandleCreateChannelGroup(c *fiber.Ctx) error {
	form := new(struct {
		GroupName string `form:"group_name"`
		GroupDesc string `form:"group_desc"`
	})
	if err := c.BodyParser(form); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("그룹 폼 입력이 잘못되었습니다.")
	}

	createdID := c.Locals("user_id").(uint64)
	sess, _ := h.store.Get(c)

	err := h.service.CreateChannelGroup(CreateGroupRequest{
		GroupName: form.GroupName,
		GroupDesc: form.GroupDesc,
	}, createdID)

	if err != nil {
		log.Errorf("채널 그룹 생성 실패: %v", err)
		sess.Set("flash_error", "그룹 생성 실패: "+err.Error())
	} else {
		sess.Set("flash_success", "채널 그룹이 성공적으로 생성되었습니다.")
	}
	sess.Save()

	return c.Redirect("/channels")
}

// HandleCreateChannelDetail은 'POST /channels/details' 요청을 처리합니다. (모달 생성)
func (h *ChannelHandler) HandleCreateChannelDetail(c *fiber.Ctx) error {
	form := new(struct {
		ChannelName string `form:"channel_name"`
		ChannelID   string `form:"channel_id"`
	})
	if err := c.BodyParser(form); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("상세 채널 폼 입력이 잘못되었습니다.")
	}

	createdID := c.Locals("user_id").(uint64)
	sess, _ := h.store.Get(c)

	err := h.service.CreateChannelDetail(CreateDetailRequest{
		ChannelName: form.ChannelName,
		ChannelID:   form.ChannelID,
	}, createdID)

	if err != nil {
		log.Errorf("상세 채널 생성 실패: %v", err)
		sess.Set("flash_error", "채널 등록 실패: "+err.Error())
	} else {
		sess.Set("flash_success", "상세 채널이 성공적으로 등록되었습니다.")
	}
	sess.Save()

	return c.Redirect("/channels")
}

// HandleUpdateMapping은 'POST /channels/map' 요청을 처리합니다. (매핑 저장)
func (h *ChannelHandler) HandleUpdateMapping(c *fiber.Ctx) error {
	type mappingForm struct {
		GroupID   uint64   `form:"group_id"`
		DetailIDs []uint64 `form:"detail_ids"`
	}
	form := new(mappingForm)
	if err := c.BodyParser(form); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("매핑 폼 입력이 잘못되었습니다.")
	}

	userID := c.Locals("user_id").(uint64)
	userRole := c.Locals("user_role").(string)
	sess, _ := h.store.Get(c)

	err := h.service.UpdateGroupMappings(form.GroupID, form.DetailIDs, userID, userRole)

	if err != nil {
		log.Errorf("채널 매핑 업데이트 실패: %v", err)
		sess.Set("flash_error", "매핑 저장 실패: "+err.Error())
	} else {
		sess.Set("flash_success", "채널 매핑이 성공적으로 저장되었습니다.")
	}
	sess.Save()

	return c.Redirect(fmt.Sprintf("/channels?group_id=%d", form.GroupID))
}

// --- (수정) '페이지 보기' 핸들러 2개 삭제 ---
// func (h *ChannelHandler) HandleShowEditGroupPage(c *fiber.Ctx) error { ... }
// func (h *ChannelHandler) HandleShowEditDetailPage(c *fiber.Ctx) error { ... }


// HandleUpdateGroup는 'POST /channels/groups/edit/:id' 요청을 처리합니다. (모달 수정)
func (h *ChannelHandler) HandleUpdateGroup(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(400).SendString("유효하지 않은 ID입니다.")
	}
	
	form := new(struct {
		GroupName string `form:"group_name"`
		GroupDesc string `form:"group_desc"`
	})
	if err := c.BodyParser(form); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("그룹 폼 입력이 잘못되었습니다.")
	}
	
	userID := c.Locals("user_id").(uint64)
	userRole := c.Locals("user_role").(string)
	sess, _ := h.store.Get(c)

	err = h.service.UpdateChannelGroup(CreateGroupRequest{
		GroupName: form.GroupName,
		GroupDesc: form.GroupDesc,
	}, uint64(id), userID, userRole)

	if err != nil {
		log.Errorf("채널 그룹 수정 실패: %v", err)
		sess.Set("flash_error", "그룹 수정 실패: "+err.Error())
	} else {
		sess.Set("flash_success", "채널 그룹(ID: "+strconv.Itoa(id)+")이 성공적으로 수정되었습니다.")
	}
	sess.Save()
	
	return c.Redirect("/channels")
}

// HandleDeleteGroup는 'POST /channels/groups/delete/:id' 요청을 처리합니다. (삭제)
func (h *ChannelHandler) HandleDeleteGroup(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(400).SendString("유효하지 않은 ID입니다.")
	}

	userID := c.Locals("user_id").(uint64)
	userRole := c.Locals("user_role").(string)
	sess, _ := h.store.Get(c)
	
	err = h.service.DeleteChannelGroup(uint64(id), userID, userRole)
	
	if err != nil {
		log.Errorf("채널 그룹 삭제 실패: %v", err)
		sess.Set("flash_error", "그룹 삭제 실패: "+err.Error())
	} else {
		sess.Set("flash_success", "채널 그룹(ID: "+strconv.Itoa(id)+")이 성공적으로 삭제되었습니다.")
	}
	sess.Save()

	return c.Redirect("/channels")
}

// HandleUpdateDetail은 'POST /channels/details/edit/:id' 요청을 처리합니다. (모달 수정)
func (h *ChannelHandler) HandleUpdateDetail(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(400).SendString("유효하지 않은 ID입니다.")
	}
	
	form := new(struct {
		ChannelName string `form:"channel_name"`
		ChannelID   string `form:"channel_id"`
	})
	if err := c.BodyParser(form); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("상세 채널 폼 입력이 잘못되었습니다.")
	}
	
	userID := c.Locals("user_id").(uint64)
	userRole := c.Locals("user_role").(string)
	sess, _ := h.store.Get(c)

	err = h.service.UpdateChannelDetail(CreateDetailRequest{
		ChannelName: form.ChannelName,
		ChannelID:   form.ChannelID,
	}, uint64(id), userID, userRole)

	if err != nil {
		log.Errorf("상세 채널 수정 실패: %v", err)
		sess.Set("flash_error", "상세 채널 수정 실패: "+err.Error())
	} else {
		sess.Set("flash_success", "상세 채널(ID: "+strconv.Itoa(id)+")이 성공적으로 수정되었습니다.")
	}
	sess.Save()
	
	return c.Redirect("/channels")
}

// HandleDeleteDetail은 'POST /channels/details/delete/:id' 요청을 처리합니다. (삭제)
func (h *ChannelHandler) HandleDeleteDetail(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(400).SendString("유효하지 않은 ID입니다.")
	}

	userID := c.Locals("user_id").(uint64)
	userRole := c.Locals("user_role").(string)
	sess, _ := h.store.Get(c)
	
	err = h.service.DeleteChannelDetail(uint64(id), userID, userRole)
	
	if err != nil {
		log.Errorf("상세 채널 삭제 실패: %v", err)
		sess.Set("flash_error", "상세 채널 삭제 실패: "+err.Error())
	} else {
		sess.Set("flash_success", "상세 채널(ID: "+strconv.Itoa(id)+")이 성공적으로 삭제되었습니다.")
	}
	sess.Save()

	return c.Redirect("/channels")
}