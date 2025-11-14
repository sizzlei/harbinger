package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal" // (우아한 종료)
	"syscall"     // (우아한 종료)
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gofiber/storage/mysql/v2" // (MySQL 스토어)
	"github.com/gofiber/template/html/v2"
	_ "github.com/go-sql-driver/mysql" // 드라이버 임포트
	log "github.com/sirupsen/logrus"   // Logrus 사용
	"github.com/sizzlei/confloader"

	// Harbinger의 내부 패키지 임포트
	"harbinger/internal/auth"
	"harbinger/internal/aws"
	"harbinger/internal/channel"
	"harbinger/internal/dashboard"
	"harbinger/internal/middleware" // (미들웨어 임포트)
	"harbinger/internal/notice"
	"harbinger/internal/scheduler" // (스케줄러 임포트)
	"harbinger/internal/slackbot"
	"harbinger/internal/template"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "conf", "/dba/service/infra/harbinger", "parameter store key")
	flag.Parse()

	// Configure File load
	config, err := confloader.AWSParamLoader("ap-northeast-2", configPath)
	if err != nil {
		log.Panic(err)
	}

	// Configure Setup
	repositoryConfig := config.Keyload("repository")

	// DB 연결
	dbo, err := aws.CreateConnection(aws.DBI{
		User:     repositoryConfig["User"].(string),
		Password: repositoryConfig["Password"].(string),
		Endpoint: repositoryConfig["Endpoint"].(string),
		Port:     repositoryConfig["Port"].(int),
		Database: repositoryConfig["Database"].(string),
	})
	if err != nil {
		log.Fatalf("Repository Connection failed. %v", err)
	}
	log.Info("Successfully connected to the database.")

	// 5. 의존성 조립 (Dependency Injection)
	sessionStore := session.New(session.Config{
		Storage: mysql.New(mysql.Config{
			Db:    dbo.DB, // (*sqlx.DB에서 표준 *sql.DB 추출)
			Table: "fiber_sessions",
		}),
		Expiration:     30 * time.Minute,
		CookieName:     "harbinger_session",
		CookieSecure:   false, 
		CookieHTTPOnly: true,
	})
	log.Info("MySQL 세션 스토어가 설정되었습니다.")

	// --- HSS 조립 ---

	// (Slackbot 스토어는 Auth 서비스보다 먼저 생성되어야 합니다)
	slackbotStore := slackbot.NewStore(dbo)

	// Auth (수정)
	authStore := auth.NewStore(dbo)
	authService := auth.NewService(authStore, slackbotStore) // (slackbotStore 주입)
	authHandler := auth.NewAuthHandler(authService, sessionStore)

	// Template
	templateStore := template.NewStore(dbo)
	templateService := template.NewService(templateStore)
	templateHandler := template.NewTemplateHandler(templateService, sessionStore)

	// Channel
	channelStore := channel.NewStore(dbo)
	channelService := channel.NewService(channelStore)
	channelHandler := channel.NewChannelHandler(channelService, sessionStore)

	// Slackbot
	slackbotService := slackbot.NewService(slackbotStore)
	slackbotHandler := slackbot.NewSlackbotHandler(slackbotService, sessionStore)

	// Notice
	noticeStore := notice.NewStore(dbo)
	noticeService := notice.NewService(noticeStore, channelStore, templateStore, slackbotStore)
	noticeHandler := notice.NewNoticeHandler(noticeService, sessionStore)

	// Dashboard
	dashboardService := dashboard.NewService(noticeStore, templateStore, channelStore)
	dashboardHandler := dashboard.NewDashboardHandler(dashboardService)

	// Scheduler
	scheduler := scheduler.NewScheduler(noticeStore, noticeService)

	// 6. Fiber 앱 생성 및 템플릿 설정
	engine := html.New("./web/views", ".html")
	engine.Reload(true) // 개발 중 캐시 끄기

	app := fiber.New(fiber.Config{
		Views: engine,
	})
	log.Info("HTML 템플릿 엔진(web/views)이 'Standard' 모드로 설정되었습니다.")

	// 7. 정적 파일(CSS, JS) 라우팅
	app.Static("/public", "./web/public")

	// 8. 라우트(URL) 설정
	log.Info("라우트를 설정합니다...")

	// 인증이 필요 *없는* 그룹
	authGroup := app.Group("/auth")
	{
		authGroup.Get("/register", authHandler.HandleShowRegisterPage)
		authGroup.Post("/register", authHandler.HandleRegister)
		authGroup.Get("/register/pending", authHandler.HandleRegisterPending)
		authGroup.Get("/login", authHandler.HandleShowLoginPage)
		authGroup.Post("/login", authHandler.HandleLogin)
		authGroup.Get("/setup-otp", authHandler.HandleShowSetupOTP)
		authGroup.Post("/setup-otp", authHandler.HandleProcessSetupOTP)
		authGroup.Get("/verify-otp", authHandler.HandleShowVerifyOTP)
		authGroup.Post("/verify-otp", authHandler.HandleProcessVerifyOTP)
	}

	app.Get("/", func(c *fiber.Ctx) error {
		return c.Redirect("/auth/login")
	})

	// --- (수정) 보호 그룹 분리 ---

	// 1. 인증이 *필요한* 그룹 (로그인한 모든 사용자: ADMIN, USERS)
	appGroup := app.Group("/", middleware.AuthMiddleware(sessionStore))
	{
		appGroup.Get("/dashboard", dashboardHandler.HandleShowDashboard)
		appGroup.Get("/auth/logout", authHandler.HandleLogout)

		// [채널 관리] (수정)
		appGroup.Get("/channels", channelHandler.HandleShowChannelPage)
		// (Groups)
		appGroup.Post("/channels/groups", channelHandler.HandleCreateChannelGroup)
		
		appGroup.Post("/channels/groups/edit/:id", channelHandler.HandleUpdateGroup)  // (신규)
		appGroup.Post("/channels/groups/delete/:id", channelHandler.HandleDeleteGroup) // (신규)
		// (Details)
		appGroup.Post("/channels/details", channelHandler.HandleCreateChannelDetail)
		
		appGroup.Post("/channels/details/edit/:id", channelHandler.HandleUpdateDetail) // (신규)
		appGroup.Post("/channels/details/delete/:id", channelHandler.HandleDeleteDetail) // (신규)
		// (Mapping)
		appGroup.Post("/channels/map", channelHandler.HandleUpdateMapping)

		// [템플릿 관리]
		appGroup.Get("/templates", templateHandler.HandleShowTemplatePage)
		appGroup.Post("/templates", templateHandler.HandleCreateTemplate)
		appGroup.Get("/templates/edit/:id", templateHandler.HandleShowEditTemplatePage)
		appGroup.Post("/templates/edit/:id", templateHandler.HandleUpdateTemplate)
		appGroup.Post("/templates/delete/:id", templateHandler.HandleDeleteTemplate)

		// [공지 스케줄 관리]
		appGroup.Get("/notices", noticeHandler.HandleShowNoticePage)
		appGroup.Post("/notices", noticeHandler.HandleCreateNotice)
		appGroup.Get("/notices/edit/:id", noticeHandler.HandleShowEditNoticePage)
		appGroup.Post("/notices/edit/:id", noticeHandler.HandleUpdateNotice)
		appGroup.Post("/notices/delete/:id", noticeHandler.HandleDeleteNotice)
		appGroup.Post("/notices/test/:id", noticeHandler.HandleTestSendNotice)

		// [Slack 봇 관리]
		appGroup.Get("/bots", slackbotHandler.HandleShowBotPage)
		appGroup.Post("/bots", slackbotHandler.HandleCreateBot)
		appGroup.Get("/bots/edit/:id", slackbotHandler.HandleShowEditBotPage)
		appGroup.Post("/bots/edit/:id", slackbotHandler.HandleUpdateBot)
		appGroup.Post("/bots/delete/:id", slackbotHandler.HandleDeleteBot)
	}

	// 2. 관리자 전용 그룹 (ADMIN만)
	adminGroup := app.Group("/admin",
		middleware.AuthMiddleware(sessionStore),
		middleware.AdminOnlyMiddleware(sessionStore),
	)
	{
		adminGroup.Get("/users", authHandler.HandleShowAdminPage)
		adminGroup.Post("/approve/:id", authHandler.HandleApproveUser)
		adminGroup.Post("/privilege", authHandler.HandleChangePrivilege)
	}

	// 9. 서버 시작 (우아한 종료 로직)

	// (스케줄러 시작)
	scheduler.Start()

	// (Fiber 앱 시작)
	go func() {
		serverPort := os.Getenv("SERVER_PORT")
		if serverPort == "" {
			serverPort = "3000"
		}
		log.Infof("Harbinger 서버(HTTP)가 [::]:%s 포트에서 시작됩니다.", serverPort)
		if err := app.Listen(fmt.Sprintf(":%s", serverPort)); err != nil {
			log.Panicf("HTTP 서버 Listen 실패: %v", err)
		}
	}()

	// (종료 신호 대기)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit

	log.Println("[INFO] Harbinger 서버 종료 신호 수신...")

	scheduler.Stop()

	if err := app.Shutdown(); err != nil {
		log.Errorf("HTTP 서버 Shutdown 실패: %v", err)
	}

	log.Println("[INFO] Harbinger 서버가 정상적으로 종료되었습니다.")
}