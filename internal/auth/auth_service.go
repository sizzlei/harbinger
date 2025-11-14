package auth

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"errors" // (errors.Is를 위해 임포트)
	"fmt"
	"image/png"
	"log"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/sync/errgroup" // (병렬 조회를 위해 임포트)
)

// LoginStatus는 로그인 상태 식별을 위한 상수입니다.
type LoginStatus int

const (
	StatusUserNotFound      LoginStatus = iota // 0: 사용자를 찾을 수 없음
	StatusPendingVerification                  // 1: 관리자 승인 대기 중
	StatusRequiresOtpSetup                     // 2: 최초 로그인 (OTP 등록 필요)
	StatusRequiresOtp                          // 3: 일반 로그인 (OTP 인증 필요)
)

// Service는 'auth' 기능의 비즈니스 로직을 담당합니다.
type Service struct {
	store *Store
}

// NewService는 Store를 받아 새 Service를 생성합니다.
func NewService(store *Store) *Service {
	return &Service{store: store}
}

// RegisterRequest는 핸들러(웹)로부터 받은 가입 요청 데이터입니다.
type RegisterRequest struct {
	UserName     string
	Email        string
	Organization string
}

// RegisterUser는 신규 사용자 가입 비즈니스 로직을 처리합니다.
func (s *Service) RegisterUser(req RegisterRequest) error {
	// 1. 핸들러로부터 받은 데이터를 DB 모델(User)로 변환
	newUser := &User{
		UserName:       req.UserName,
		Email:          req.Email,
		PrivilegesType: "USERS", // 신규 가입자 기본 권한
		VerifyYn:       false,    // 관리자 승인 대기
	}

	// 2. Organization 값이 비어있지 않은 경우에만 포인터 할당
	if req.Organization != "" {
		newUser.Organization = &req.Organization
	}
	// OtpCode는 기본값(nil)이므로 NULL로 INSERT 됩니다.

	// 3. Store를 호출하여 DB에 저장
	err := s.store.CreateUser(newUser)
	if err != nil {
		log.Printf("[ERROR] RegisterUser 서비스 에러: %v", err)
		return err
	}

	return nil
}

// CheckLoginStatus는 이메일을 받아 사용자의 로그인 상태를 3가지로 분기합니다.
func (s *Service) CheckLoginStatus(email string) (LoginStatus, *User, error) {
	// 1. Store를 통해 이메일로 사용자 조회
	user, err := s.store.GetUserByEmail(email)
	if err != nil {
		// (DB 자체 에러)
		log.Printf("[ERROR] CheckLoginStatus에서 DB 조회 실패: %v", err)
		return StatusUserNotFound, nil, err
	}

	// 2. (분기 1) 사용자가 존재하지 않음
	if user == nil {
		log.Printf("[INFO] 로그인 실패: 존재하지 않는 이메일 (%s)", email)
		return StatusUserNotFound, nil, nil
	}

	// 3. (분기 2) 사용자는 있으나, 관리자 승인 대기 중
	if !user.VerifyYn { // (VerifyYn == false)
		log.Printf("[INFO] 로그인 거부: 승인 대기 (%s)", email)
		return StatusPendingVerification, nil, nil
	}

	// 4. (분기 3) 승인됨 & 최초 로그인 (OTP 등록 필요)
	if user.OtpCode == nil {
		log.Printf("[INFO] 로그인 시도: 최초 로그인 (OTP 등록 필요) (%s)", email)
		return StatusRequiresOtpSetup, user, nil
	}

	// 5. (분기 4) 승인됨 & 일반 로그인 (OTP 인증 필요)
	log.Printf("[INFO] 로그인 시도: 일반 로그인 (OTP 인증 필요) (%s)", email)
	return StatusRequiresOtp, user, nil
}

// GenerateOTP는 신규 사용자를 위한 (1)Base32 비밀 키, (2)Base64 QR 이미지를 생성합니다.
func (s *Service) GenerateOTP(email string) (string, string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "Harbinger",
		AccountName: email,
	})
	if err != nil {
		log.Printf("[ERROR] TOTP Key 생성 실패: %v", err)
		return "", "", err
	}

	// 1. Base32 비밀 키 (DB/세션 저장용)
	secretKey := key.Secret() // 예: "JBSWY3DPEHPK3PXP"

	// 2. Base64 QR 이미지 (HTML <img> 태그용)
	var buf bytes.Buffer
	img, err := key.Image(200, 200)
	if err != nil {
		log.Printf("[ERROR] TOTP QR 이미지 생성 실패: %v", err)
		return "", "", err
	}
	png.Encode(&buf, img)
	qrImageBase64 := base64.StdEncoding.EncodeToString(buf.Bytes()) // 예: "iVBORw0KGgo..."

	// (반환 순서: 1. 비밀 키, 2. 이미지)
	return secretKey, qrImageBase64, nil
}

// ValidateOTP는 시간 오차를 허용하여 코드를 검증합니다.
func (s *Service) ValidateOTP(passcode string, secretKey string) bool {
	// 'Skew: 1' 옵션은 1 * 30초 = 총 90초(과거 30, 현재 30, 미래 30)의
	// 시간 오차를 허용하여, 서버-클라이언트 간의 시간 불일치 문제를 해결합니다.
	opts := totp.ValidateOpts{
		Period:    30,                      // 30초 주기
		Skew:      1,                       // 1 주기(30초)의 오차 허용
		Digits:    otp.DigitsSix,           // 6자리
		Algorithm: otp.AlgorithmSHA1,       // 표준 알고리즘
	}

	valid, err := totp.ValidateCustom(passcode, secretKey, time.Now(), opts)
	if err != nil {
		// (중요) Base32 디코딩 실패 시 이 에러가 발생합니다.
		log.Printf("[WARN] OTP 검증 중 에러: %v", err)
		return false
	}

	return valid
}

// FinalizeOTPSetup은 'secretKey'를 DB에 영구 저장합니다.
func (s *Service) FinalizeOTPSetup(email string, secretKey string) error {
	// 1. Store를 호출하여 DB에 'otp_code' 업데이트
	err := s.store.UpdateUserOTP(email, secretKey)
	if err != nil {
		return err
	}
	return nil
}

// AdminPageData는 관리자 페이지에 필요한 모든 데이터를 담습니다.
type AdminPageData struct {
	PendingUsers  []User // 승인 대기 사용자
	VerifiedUsers []User // 승인된 사용자
}

// GetAdminPageData는 관리자 페이지의 모든 데이터를 병렬로 조회합니다.
func (s *Service) GetAdminPageData(adminRole string) (*AdminPageData, error) {
	// 1. (권한 확인)
	if adminRole != "ADMIN" {
		return nil, fmt.Errorf("권한 없음: 관리자만 이 데이터를 조회할 수 있습니다.")
	}

	var data AdminPageData
	var eg errgroup.Group

	// 고루틴 1: 승인 대기 사용자 조회
	eg.Go(func() error {
		users, err := s.store.GetPendingUsers()
		if err != nil {
			log.Printf("[ERROR] GetPendingUsers 서비스 에러: %v", err)
			return err
		}
		data.PendingUsers = users
		return nil
	})

	// 고루틴 2: 승인된 사용자 조회
	eg.Go(func() error {
		users, err := s.store.GetAllVerifiedUsers()
		if err != nil {
			log.Printf("[ERROR] GetAllVerifiedUsers 서비스 에러: %v", err)
			return err
		}
		data.VerifiedUsers = users
		return nil
	})

	// 두 쿼리가 모두 완료될 때까지 대기
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	
	return &data, nil
}


// ApproveUser는 관리자가 특정 사용자를 승인하는 로직입니다.
func (s *Service) ApproveUser(adminRole string, userIDToApprove uint64) error {
	// 1. (권한 확인) 호출자가 ADMIN인지 확인 (중요)
	if adminRole != "ADMIN" {
		return fmt.Errorf("권한 없음: 사용자 승인은 관리자(ADMIN)만 가능합니다.")
	}
	
	// 2. 스토어 호출
	err := s.store.ApproveUser(userIDToApprove)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("사용자(ID: %d)를 찾을 수 없거나 이미 승인되었습니다.", userIDToApprove)
	}
	return err
}

// (신규) ChangeUserPrivilege는 관리자가 사용자의 권한을 변경합니다.
func (s *Service) ChangeUserPrivilege(adminRole string, userIDToChange uint64, newRole string) error {
	// 1. (권한 확인)
	if adminRole != "ADMIN" {
		return fmt.Errorf("권한 없음: 관리자만 권한을 변경할 수 있습니다.")
	}
	
	// 2. (유효성 검사)
	if newRole != "ADMIN" && newRole != "USERS" {
		return fmt.Errorf("유효하지 않은 권한입니다: %s", newRole)
	}
	
	// (TODO: 자기 자신의 권한을 변경하지 못하도록 막는 로직 추가 필요)
	// if adminID == userIDToChange { ... }

	// 3. 스토어 호출
	err := s.store.UpdateUserPrivilege(userIDToChange, newRole)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("사용자(ID: %d)를 찾을 수 없습니다.", userIDToChange)
	}
	return err
}