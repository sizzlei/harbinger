package dashboard

import (
	"github.com/gofiber/fiber/v2"
	
	log "github.com/sirupsen/logrus"
)

// (수정) DashboardData, Service, NewService, GetDashboardData 정의 (제거)
// (이 정의들은 'dashboard_service.go'에 있습니다.)

// DashboardHandler는 대시보드 관련 핸들러입니다.
type DashboardHandler struct {
	service *Service // (dashboard_service.go에 정의됨)
}

// NewDashboardHandler는 새 핸들러를 생성합니다.
func NewDashboardHandler(service *Service) *DashboardHandler {
	return &DashboardHandler{service: service}
}

// HandleShowDashboard는 'GET /dashboard' 요청을 처리합니다.
func (h *DashboardHandler) HandleShowDashboard(c *fiber.Ctx) error {
	// 1. (수정) 미들웨어에서 'user_id'와 'user_role' 값을 Locals에서 꺼냅니다.
	userEmail := c.Locals("user_email").(string)
	userRole := c.Locals("user_role").(string)
	userID := c.Locals("user_id").(uint64)

	// 2. (수정) 서비스 호출 시 권한 인자 전달
	data, err := h.service.GetDashboardData(userID, userRole)
	if err != nil {
		log.Errorf("대시보드 데이터 조회 실패: %v", err)
		return c.Status(500).SendString("데이터 조회 중 오류 발생")
	}

	// 3. 'dashboard.html' 뷰(View)에 데이터를 전달하여 렌더링
	return c.Render("dashboard", fiber.Map{
		"Title":     "Harbinger | 대시보드",
		"Data":      data,
		"UserEmail": userEmail,
		"UserRole":  userRole,
	}, "layout")
}