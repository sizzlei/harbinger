package auth

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	log "github.com/sirupsen/logrus" // (logrus 표준 사용)
)

// AuthHandler
type AuthHandler struct {
	service *Service
	store   *session.Store
}

// NewAuthHandler
func NewAuthHandler(service *Service, store *session.Store) *AuthHandler {
	return &AuthHandler{
		service: service,
		store:   store,
	}
}

// --- [가입] 플로우 ---
// HandleShowRegisterPage (수정: 플래시 메시지 로직 제거)
func (h *AuthHandler) HandleShowRegisterPage(c *fiber.Ctx) error {
	return c.Render("register", fiber.Map{
		"Title": "Harbinger | 회원가입",
		// (에러는 POST 핸들러가 직접 전달하므로 FlashError 제거)
	}, "layout")
}

// HandleRegister (수정: 에러 발생 시 Render 사용)
func (h *AuthHandler) HandleRegister(c *fiber.Ctx) error {
	type registerForm struct {
		UserName     string `form:"user_name"`
		Email        string `form:"email"`
		Organization string `form:"organization"`
	}
	form := new(registerForm)

	if err := c.BodyParser(form); err != nil {
		log.Warnf("회원가입 폼 파싱 실패: %v", err)
		return c.Status(fiber.StatusBadRequest).SendString("입력 값이 올바르지 않습니다.")
	}
	if form.Email == "" || form.UserName == "" {
		return c.Status(fiber.StatusBadRequest).SendString("필수 값을 입력하세요.")
	}
	log.Infof("신규 가입 요청 (핸들러): %s", form.Email)

	// 2. 서비스 호출 (Slack 이메일 검증 포함)
	err := h.service.RegisterUser(RegisterRequest{
		UserName:     form.UserName,
		Email:        form.Email,
		Organization: form.Organization,
	})

	// (수정) 3. 에러 발생 시, 'Render'를 사용하여 폼 데이터와 에러 메시지 전달
	if err != nil {
		log.Warnf("가입 처리 실패: %v", err)
		// (Redirect 대신 Render 사용)
		return c.Render("register", fiber.Map{
			"Title":      "Harbinger | 회원가입",
			"FlashError": "가입 실패: " + err.Error(), // (에러 메시지 전달)
			"Form":       form,                       // (입력한 폼 데이터 다시 전달)
		}, "layout")
	}

	// (수정) 4. 성공 시 pending 페이지로 이동 (플래시 메시지 사용)
	sess, _ := h.store.Get(c)
	sess.Set("flash_success", "가입 신청이 완료되었습니다. 관리자 승인을 기다려주세요.")
	sess.Save()
	return c.Redirect("/auth/register/pending")
}

func (h *AuthHandler) HandleRegisterPending(c *fiber.Ctx) error {
	return c.Render("register_pending", fiber.Map{
		"Title": "Harbinger | 가입 신청 완료",
	}, "layout")
}

// --- [로그인] 플로우 ---
// (HandleShowLoginPage, HandleLogin - 변경 없음)

func (h *AuthHandler) HandleShowLoginPage(c *fiber.Ctx) error {
	return c.Render("login", fiber.Map{
		"Title": "Harbinger | 로그인",
	}, "layout")
}

func (h *AuthHandler) HandleLogin(c *fiber.Ctx) error {
	type loginForm struct {
		Email string `form:"email"`
	}
	form := new(loginForm)
	if err := c.BodyParser(form); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("입력 값이 올바르지 않습니다.")
	}

	status, user, err := h.service.CheckLoginStatus(form.Email)
	if err != nil {
		return c.Render("login", fiber.Map{
			"Title": "Harbinger | 로그인",
			"Error": "로그인 처리 중 서버 오류가 발생했습니다.",
		}, "layout")
	}

	sess, err := h.store.Get(c)
	if err != nil {
		log.Errorf("세션 가져오기 실패: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("세션 오류")
	}

	switch status {
	case StatusUserNotFound:
		return c.Render("login", fiber.Map{
			"Title": "Harbinger | 로그인",
			"Error": "등록되지 않은 이메일입니다.",
		}, "layout")

	case StatusPendingVerification:
		log.Infof("로그인 거부: 승인 대기 (%s)", form.Email)
		return c.Render("login", fiber.Map{
			"Title": "Harbinger | 로그인",
			"Error": "계정이 아직 관리자 승인 대기 중입니다. 승인 후 다시 시도해 주세요.",
		}, "layout")

	case StatusRequiresOtpSetup:
		sess.Set("otp_setup_email", user.Email)
		if err := sess.Save(); err != nil {
			log.Errorf("세션 저장 실패 (otp_setup): %v", err)
			return c.Status(fiber.StatusInternalServerError).SendString("세션 저장 오류")
		}
		log.Infof("세션 저장: 'otp_setup_email' = %s", user.Email)
		return c.Redirect("/auth/setup-otp")

	case StatusRequiresOtp:
		sess.Set("otp_verify_email", user.Email)
		if err := sess.Save(); err != nil {
			log.Errorf("세션 저장 실패 (otp_verify): %v", err)
			return c.Status(fiber.StatusInternalServerError).SendString("세션 저장 오류")
		}
		log.Infof("세션 저장: 'otp_verify_email' = %s", user.Email)
		return c.Redirect("/auth/verify-otp")

	default:
		return c.Status(fiber.StatusInternalServerError).SendString("알 수 없는 로그인 상태")
	}
}

// --- [OTP 최초 등록] 플로우 ---
// (HandleShowSetupOTP, HandleProcessSetupOTP - 변경 없음)

func (h *AuthHandler) HandleShowSetupOTP(c *fiber.Ctx) error {
	sess, err := h.store.Get(c)
	if err != nil {
		log.Errorf("세션 가져오기 실패 (setup-otp): %v", err)
		return c.Redirect("/auth/login")
	}

	emailInterface := sess.Get("otp_setup_email")
	if emailInterface == nil {
		log.Warn("'otp_setup_email' 세션 값이 없습니다.")
		return c.Redirect("/auth/login")
	}
	email := emailInterface.(string)

	flashError := sess.Get("flash_error")
	var errorMsg string
	if flashError != nil {
		errorMsg = flashError.(string)
		sess.Delete("flash_error")
		if err := sess.Save(); err != nil {
			log.Errorf("플래시 에러 삭제 세션 저장 실패: %v", err)
		}
	}

	dbSecretKey, qrImageString, err := h.service.GenerateOTP(email)
	if err != nil {
		log.Errorf("GenerateOTP 서비스 실패: %v", err)
		return c.Render("login", fiber.Map{"Error": "OTP 생성 실패"}, "layout")
	}

	sess.Set("otp_setup_secret", dbSecretKey)
	if err := sess.Save(); err != nil {
		log.Errorf("세션 저장 실패 (otp_setup_secret): %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("세션 저장 오류")
	}

	return c.Render("setup_otp", fiber.Map{
		"Title":       "Harbinger | OTP 등록",
		"Email":       email,
		"QRCodeImage": qrImageString,
		"Error":       errorMsg,
	}, "layout")
}

func (h *AuthHandler) HandleProcessSetupOTP(c *fiber.Ctx) error {
	type otpForm struct {
		OtpToken string `form:"otp_token"`
	}
	form := new(otpForm)
	if err := c.BodyParser(form); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("입력 값이 올바르지 않습니다.")
	}

	sess, err := h.store.Get(c)
	if err != nil {
		log.Errorf("세션 가져오기 실패 (process-setup-otp): %v", err)
		return c.Redirect("/auth/login")
	}

	email := sess.Get("otp_setup_email")
	secret := sess.Get("otp_setup_secret")

	if email == nil || secret == nil {
		log.Warn("OTP 등록 세션 값이 없습니다. (email 또는 secret 누락)")
		return c.Redirect("/auth/login")
	}
	emailStr := email.(string)
	secretStr := secret.(string)

	isValid := h.service.ValidateOTP(form.OtpToken, secretStr)

	if !isValid {
		log.Warnf("OTP 코드 검증 실패: %s", emailStr)
		sess.Set("flash_error", "인증 코드가 올바르지 않습니다. 다시 시도해 주세요.")
		if err := sess.Save(); err != nil {
			log.Errorf("플래시 에러 저장 실패: %v", err)
		}
		return c.Redirect("/auth/setup-otp")
	}

	err = h.service.FinalizeOTPSetup(emailStr, secretStr)
	if err != nil {
		log.Errorf("FinalizeOTPSetup 서비스 실패: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("OTP 저장 중 오류 발생")
	}

	user, err := h.service.store.GetUserByEmail(emailStr)
	if user == nil || err != nil {
		log.Errorf("OTP 등록 후 사용자 정보 조회 실패: %v", err)
		return c.Redirect("/auth/login")
	}

	sess.Delete("otp_setup_email")
	sess.Delete("otp_setup_secret")
	sess.Set("logged_in_email", user.Email)
	sess.Set("user_id", user.ID)
	sess.Set("privileges_type", user.PrivilegesType)
	if err := sess.Save(); err != nil {
		log.Errorf("최종 로그인 세션 저장 실패: %v", err)
	}

	log.Infof("최초 OTP 등록 및 로그인 성공: %s", emailStr)

	return c.Redirect("/dashboard")
}

// --- [일반 OTP 인증] 플로우 ---
// (HandleShowVerifyOTP, HandleProcessVerifyOTP - 변경 없음)

func (h *AuthHandler) HandleShowVerifyOTP(c *fiber.Ctx) error {
	sess, err := h.store.Get(c)
	if err != nil {
		log.Errorf("세션 가져오기 실패 (show-verify-otp): %v", err)
		return c.Redirect("/auth/login")
	}

	emailInterface := sess.Get("otp_verify_email")
	if emailInterface == nil {
		log.Warn("'otp_verify_email' 세션 값이 없습니다.")
		return c.Redirect("/auth/login")
	}
	email := emailInterface.(string)

	flashError := sess.Get("flash_error")
	var errorMsg string
	if flashError != nil {
		errorMsg = flashError.(string)
		sess.Delete("flash_error")
		if err := sess.Save(); err != nil {
			log.Errorf("플래시 에러 삭제 세션 저장 실패: %v", err)
		}
	}

	return c.Render("verify_otp", fiber.Map{
		"Title": "Harbinger | 2단계 인증",
		"Email": email,
		"Error": errorMsg,
	}, "layout")
}

func (h *AuthHandler) HandleProcessVerifyOTP(c *fiber.Ctx) error {
	type otpForm struct {
		OtpToken string `form:"otp_token"`
	}
	form := new(otpForm)
	if err := c.BodyParser(form); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("입력 값이 올바르지 않습니다.")
	}

	sess, err := h.store.Get(c)
	if err != nil {
		log.Errorf("세션 가져오기 실패 (process-verify-otp): %v", err)
		return c.Redirect("/auth/login")
	}

	email := sess.Get("otp_verify_email")
	if email == nil {
		log.Warn("OTP 인증 세션 값이 없습니다. (email 누락)")
		return c.Redirect("/auth/login")
	}
	emailStr := email.(string)

	user, err := h.service.store.GetUserByEmail(emailStr)
	if user == nil || err != nil || user.OtpCode == nil {
		log.Errorf("OTP 인증 중 사용자/OTP코드를 찾을 수 없음: %s", emailStr)
		return c.Redirect("/auth/login")
	}
	secretStr := *user.OtpCode

	isValid := h.service.ValidateOTP(form.OtpToken, secretStr)

	if !isValid {
		log.Warnf("일반 OTP 코드 검증 실패: %s", emailStr)
		sess.Set("flash_error", "인증 코드가 올바르지 않습니다.")
		if err := sess.Save(); err != nil {
			log.Errorf("플래시 에러 저장 실패: %v", err)
		}
		return c.Redirect("/auth/verify-otp")
	}

	sess.Delete("otp_verify_email")
	sess.Set("logged_in_email", emailStr)
	sess.Set("user_id", user.ID)
	sess.Set("privileges_type", user.PrivilegesType) 
	if err := sess.Save(); err != nil {
		log.Errorf("최종 로그인 세션 저장 실패: %v", err)
	}

	log.Infof("일반 OTP 인증 및 로그인 성공: %s", emailStr)

	return c.Redirect("/dashboard")
}

// --- [로그아웃] 플로우 ---
// (HandleLogout - 변경 없음)

func (h *AuthHandler) HandleLogout(c *fiber.Ctx) error {
	sess, err := h.store.Get(c)
	if err != nil {
		log.Errorf("로그아웃: 세션 가져오기 실패: %v", err)
		return c.Redirect("/auth/login")
	}

	if err := sess.Destroy(); err != nil {
		log.Errorf("로그아웃: 세션 파기 실패: %v", err)
		return c.Status(500).SendString("로그아웃 처리 중 오류 발생")
	}

	log.Info("사용자 로그아웃 성공")

	return c.Redirect("/auth/login")
}

// --- [관리자 기능] ---

// HandleShowAdminPage는 'GET /admin/users' 요청을 처리합니다.
func (h *AuthHandler) HandleShowAdminPage(c *fiber.Ctx) error {
	sess, _ := h.store.Get(c)
	
	// 1. 플래시 메시지 읽기
	flashSuccess := sess.Get("flash_success"); if flashSuccess != nil { sess.Delete("flash_success") }
	flashError := sess.Get("flash_error");   if flashError != nil { sess.Delete("flash_error") }
	sess.Save()
	
	// 2. 서비스 호출 (미승인/승인 사용자 목록 병렬 조회)
	adminRole := c.Locals("user_role").(string) 
	data, err := h.service.GetAdminPageData(adminRole)
	if err != nil {
		log.Errorf("관리자 페이지 사용자 조회 실패: %v", err)
		return c.Status(500).SendString("데이터 조회 중 오류 발생")
	}
	
	// 3. (수정) Locals에서 UserRole 가져오기
	userEmail := c.Locals("user_email").(string)

	// 4. 'admin_users.html' 뷰(View) 렌더링
	return c.Render("admin_users", fiber.Map{
		"Title": "Harbinger | 사용자 승인 관리",
		"UserEmail": userEmail,
		"UserRole":  adminRole, // (layout.html이 사용할 수 있도록 역할 전달)
		"PendingUsers": data.PendingUsers,  // 승인 대기 목록
		"VerifiedUsers": data.VerifiedUsers, // 승인된 사용자 목록
		"FlashSuccess": flashSuccess,
		"FlashError":   flashError,
	}, "layout")
}

// HandleApproveUser는 'POST /admin/approve/:id' 요청을 처리합니다.
func (h *AuthHandler) HandleApproveUser(c *fiber.Ctx) error {
	userIDToApprove, err := c.ParamsInt("id")
	if err != nil || userIDToApprove <= 0 {
		return c.Status(400).SendString("유효하지 않은 사용자 ID입니다.")
	}

	adminRole := c.Locals("user_role").(string)
	sess, _ := h.store.Get(c)

	err = h.service.ApproveUser(adminRole, uint64(userIDToApprove))

	if err != nil {
		log.Errorf("사용자 승인 실패 (ID: %d): %v", userIDToApprove, err)
		sess.Set("flash_error", "승인 실패: "+err.Error())
	} else {
		sess.Set("flash_success", "사용자(ID: "+strconv.Itoa(userIDToApprove)+")가 성공적으로 승인되었습니다.")
	}
	sess.Save()

	return c.Redirect("/admin/users")
}

// HandleChangePrivilege는 'POST /admin/privilege' 요청을 처리합니다.
func (h *AuthHandler) HandleChangePrivilege(c *fiber.Ctx) error {
	type privilegeForm struct {
		UserID  uint64 `form:"user_id"`
		NewRole string `form:"new_role"`
	}
	form := new(privilegeForm)
	if err := c.BodyParser(form); err != nil {
		return c.Status(400).SendString("권한 변경 폼 입력이 잘못되었습니다.")
	}

	adminRole := c.Locals("user_role").(string)
	sess, _ := h.store.Get(c)

	err := h.service.ChangeUserPrivilege(adminRole, form.UserID, form.NewRole)

	if err != nil {
		log.Errorf("사용자 권한 변경 실패 (ID: %d): %v", form.UserID, err)
		sess.Set("flash_error", "권한 변경 실패: "+err.Error())
	} else {
		sess.Set("flash_success", "사용자(ID: "+strconv.FormatUint(form.UserID, 10)+")의 권한이 "+form.NewRole+"(으)로 변경되었습니다.")
	}
	sess.Save()
	
	return c.Redirect("/admin/users")
}