package middleware

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
)

// AdminOnlyMiddleware는 'AuthMiddleware' (로그인 확인) *다음에* 실행되어야 하며,
// 세션에 저장된 'user_role'이 'ADMIN'인지 확인합니다.
func AdminOnlyMiddleware(store *session.Store) fiber.Handler {

	return func(c *fiber.Ctx) error {
		// 1. (AuthMiddleware가 이미 실행했다고 가정하고) c.Locals에서 역할을 가져옵니다.
		roleInterface := c.Locals("user_role")

		// 2. 역할이 없거나 'ADMIN'이 아닌 경우
		if roleInterface == nil || roleInterface.(string) != "ADMIN" {
			log.Printf("[WARN] [Admin] 권한 없는 접근 (Role: %v, Path: %s)", roleInterface, c.Path())
			
			// (참고: 에러 페이지 대신 대시보드로 리다"이렉트)
			return c.Redirect("/dashboard") 
		}

		// 3. (ADMIN 확인) 다음 핸들러로 통과
		log.Printf("[INFO] [Admin] 관리자 접근 허용 (Path: %s)", c.Path())
		return c.Next()
	}
}