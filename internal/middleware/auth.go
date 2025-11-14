package middleware

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
)

func AuthMiddleware(store *session.Store) fiber.Handler {

	return func(c *fiber.Ctx) error {
		sess, err := store.Get(c)
		if err != nil { /* ... */ }

		// 'logged_in_email'과 'user_id'를 모두 확인
		emailInterface := sess.Get("logged_in_email")
		userIDInterface := sess.Get("user_id")
		roleInterface := sess.Get("privileges_type") // (수정)

		if emailInterface == nil || userIDInterface == nil || roleInterface == nil { // (수정)
			log.Printf("[WARN] 미들웨어: 로그인되지 않은 접근 (%s)", c.Path())
			return c.Redirect("/auth/login")
		}

		// (수정) c.Locals에 user_id와 user_role 저장
		c.Locals("user_email", emailInterface.(string))
		c.Locals("user_id", userIDInterface.(uint64))
		c.Locals("user_role", roleInterface.(string)) // (수정)
		
		log.Printf("[INFO] 미들웨어: 인증된 접근 (%s)", c.Path())
		return c.Next()
	}
}