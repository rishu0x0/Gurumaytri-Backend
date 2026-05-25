package auth

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"school-management/backend/internal/msg91"
	jwtpkg "school-management/backend/pkg/jwt"
)

var mobileRe = regexp.MustCompile(`^\d{10}$`)

type Service struct {
	db     *pgxpool.Pool
	jwtSvc *jwtpkg.Service
	msg91  *msg91.Client
}

func NewService(db *pgxpool.Pool, jwtSvc *jwtpkg.Service, m *msg91.Client) *Service {
	return &Service{db: db, jwtSvc: jwtSvc, msg91: m}
}

// SendRegistrationOTP validates fields, checks for duplicate mobile, sends OTP, stores session.
// Returns the MSG91 reqId. Mobile must be 10 digits (no country code — we prepend 91).
func (s *Service) SendRegistrationOTP(ctx context.Context, name, mobile, schoolName, password string) (string, error) {
	if err := validateRegistrationFields(name, mobile, schoolName, password); err != nil {
		return "", err
	}

	// Check duplicate mobile
	var exists bool
	err := s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM teachers WHERE mobile = $1)`, mobile,
	).Scan(&exists)
	if err != nil {
		return "", fmt.Errorf("db check: %w", err)
	}
	if exists {
		return "", ErrDuplicateMobile
	}

	// Send OTP via MSG91 (prepend country code 91)
	reqID, err := s.msg91.SendOTP(ctx, "91"+mobile)
	if err != nil {
		return "", fmt.Errorf("send otp: %w", err)
	}

	// Store OTP session (expires in 10 minutes, set in migration default)
	_, err = s.db.Exec(ctx,
		`INSERT INTO otp_sessions (mobile, req_id) VALUES ($1, $2)`,
		mobile, reqID,
	)
	if err != nil {
		return "", fmt.Errorf("store otp session: %w", err)
	}

	return reqID, nil
}

// VerifyRegistrationOTP verifies the OTP with MSG91, creates the teacher, and returns tokens.
func (s *Service) VerifyRegistrationOTP(ctx context.Context, reqID, otp, name, mobile, schoolName, password string) (accessToken, refreshToken string, err error) {
	// Load and validate OTP session
	var sessionMobile string
	var attemptCount int
	var expiresAt time.Time
	var isUsed bool

	err = s.db.QueryRow(ctx,
		`SELECT mobile, attempt_count, expires_at, is_used FROM otp_sessions WHERE req_id = $1`,
		reqID,
	).Scan(&sessionMobile, &attemptCount, &expiresAt, &isUsed)
	if err != nil {
		return "", "", ErrSessionNotFound
	}
	if isUsed {
		return "", "", ErrSessionAlreadyUsed
	}
	if time.Now().After(expiresAt) {
		return "", "", ErrOTPExpired
	}
	if attemptCount >= 3 {
		return "", "", ErrOTPLocked
	}

	// Verify OTP with MSG91
	if err = s.msg91.VerifyOTP(ctx, reqID, otp); err != nil {
		// Increment attempt count on wrong OTP
		_, _ = s.db.Exec(ctx,
			`UPDATE otp_sessions SET attempt_count = attempt_count + 1 WHERE req_id = $1`,
			reqID,
		)
		if attemptCount+1 >= 3 {
			return "", "", ErrOTPLocked
		}
		return "", "", ErrOTPInvalid
	}

	// Mark session as used
	_, err = s.db.Exec(ctx, `UPDATE otp_sessions SET is_used = TRUE WHERE req_id = $1`, reqID)
	if err != nil {
		return "", "", fmt.Errorf("mark session used: %w", err)
	}

	// Hash password at bcrypt cost 12
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", "", fmt.Errorf("hash password: %w", err)
	}

	// Create teacher
	var teacherID string
	err = s.db.QueryRow(ctx,
		`INSERT INTO teachers (name, mobile, school_name, password_hash, is_verified)
		 VALUES ($1, $2, $3, $4, TRUE)
		 RETURNING id`,
		name, mobile, schoolName, string(hash),
	).Scan(&teacherID)
	if err != nil {
		return "", "", fmt.Errorf("create teacher: %w", err)
	}

	return s.issueTokens(ctx, teacherID, mobile, schoolName, "")
}

// RetryOTP triggers a resend of the OTP via MSG91, enforcing a 60-second cooldown between retries.
func (s *Service) RetryOTP(ctx context.Context, reqID string, retryChannel int) error {
	var expiresAt time.Time
	var isUsed bool
	var lastRetryAt *time.Time

	err := s.db.QueryRow(ctx,
		`SELECT expires_at, is_used, last_retry_at FROM otp_sessions WHERE req_id = $1`,
		reqID,
	).Scan(&expiresAt, &isUsed, &lastRetryAt)
	if err != nil {
		return ErrSessionNotFound
	}
	if isUsed || time.Now().After(expiresAt) {
		return ErrSessionExpiredOrUsed
	}

	// Enforce 60-second cooldown between retries
	if lastRetryAt != nil && time.Since(*lastRetryAt) < 60*time.Second {
		return ErrRetryTooSoon
	}

	if err = s.msg91.RetryOTP(ctx, reqID, retryChannel); err != nil {
		return fmt.Errorf("retry otp: %w", err)
	}

	_, _ = s.db.Exec(ctx,
		`UPDATE otp_sessions SET last_retry_at = NOW() WHERE req_id = $1`, reqID,
	)
	return nil
}

// issueTokens generates an access token and refresh token, stores the refresh token hash.
func (s *Service) issueTokens(ctx context.Context, teacherID, mobile, schoolName, deviceHint string) (accessToken, refreshToken string, err error) {
	accessToken, err = s.jwtSvc.GenerateAccessToken(teacherID, mobile, schoolName)
	if err != nil {
		return "", "", fmt.Errorf("generate access token: %w", err)
	}

	rawRefresh, hashRefresh, err := jwtpkg.GenerateRefreshToken()
	if err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}

	_, err = s.db.Exec(ctx,
		`INSERT INTO refresh_tokens (teacher_id, token_hash, device_hint, last_used_at)
		 VALUES ($1, $2, $3, NOW())`,
		teacherID, hashRefresh, nullableString(deviceHint),
	)
	if err != nil {
		return "", "", fmt.Errorf("store refresh token: %w", err)
	}

	return accessToken, rawRefresh, nil
}

func validateRegistrationFields(name, mobile, schoolName, password string) error {
	name = strings.TrimSpace(name)
	schoolName = strings.TrimSpace(schoolName)

	if name == "" {
		return &ValidationError{Field: "name", Message: "name is required"}
	}
	if !mobileRe.MatchString(mobile) {
		return &ValidationError{Field: "mobile", Message: "mobile must be 10 digits"}
	}
	if schoolName == "" {
		return &ValidationError{Field: "school_name", Message: "school_name is required"}
	}
	if len(password) < 8 {
		return &ValidationError{Field: "password", Message: "password must be at least 8 characters"}
	}
	if !hasUpperAndDigit(password) {
		return &ValidationError{Field: "password", Message: "password must contain at least one uppercase letter and one digit"}
	}
	return nil
}

func hasUpperAndDigit(s string) bool {
	var hasUpper, hasDigit bool
	for _, r := range s {
		if unicode.IsUpper(r) {
			hasUpper = true
		}
		if unicode.IsDigit(r) {
			hasDigit = true
		}
	}
	return hasUpper && hasDigit
}

func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// RegisterDirect creates a teacher account immediately without OTP verification.
func (s *Service) RegisterDirect(ctx context.Context, name, mobile, schoolName, password string) (accessToken, refreshToken, teacherID string, err error) {
	if err = validateRegistrationFields(name, mobile, schoolName, password); err != nil {
		return "", "", "", err
	}

	var exists bool
	if err = s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM teachers WHERE mobile = $1)`, mobile,
	).Scan(&exists); err != nil {
		return "", "", "", fmt.Errorf("db check: %w", err)
	}
	if exists {
		return "", "", "", ErrDuplicateMobile
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", "", "", fmt.Errorf("hash password: %w", err)
	}

	if err = s.db.QueryRow(ctx,
		`INSERT INTO teachers (name, mobile, school_name, password_hash, is_verified)
		 VALUES ($1, $2, $3, $4, TRUE)
		 RETURNING id`,
		name, mobile, schoolName, string(hash),
	).Scan(&teacherID); err != nil {
		return "", "", "", fmt.Errorf("create teacher: %w", err)
	}

	at, rt, err := s.issueTokens(ctx, teacherID, mobile, schoolName, "")
	return at, rt, teacherID, err
}

// Login verifies mobile + password and issues tokens on success.
// Returns ErrInvalidCredentials for any invalid combination (never reveals which field failed).
func (s *Service) Login(ctx context.Context, mobile, password, deviceHint string) (accessToken, refreshToken string, err error) {
	if !mobileRe.MatchString(mobile) {
		return "", "", ErrInvalidCredentials
	}

	var teacherID, schoolName, passwordHash string
	var isVerified bool
	err = s.db.QueryRow(ctx,
		`SELECT id, school_name, password_hash, is_verified FROM teachers WHERE mobile = $1`,
		mobile,
	).Scan(&teacherID, &schoolName, &passwordHash, &isVerified)
	if err != nil {
		// Teacher not found — return same error as wrong password
		return "", "", ErrInvalidCredentials
	}
	if !isVerified {
		return "", "", ErrInvalidCredentials
	}

	if err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return "", "", ErrInvalidCredentials
	}

	return s.issueTokens(ctx, teacherID, mobile, schoolName, deviceHint)
}

// Refresh validates the incoming refresh token and issues a new access token.
// The refresh token row is updated with last_used_at but NOT rotated (non-expiring by design).
func (s *Service) Refresh(ctx context.Context, rawRefreshToken string) (accessToken string, err error) {
	if rawRefreshToken == "" {
		return "", ErrInvalidCredentials
	}

	tokenHash := jwtpkg.HashToken(rawRefreshToken)

	// Two-step: update last_used_at and get teacher_id, then fetch mobile/school_name
	var teacherID string
	err = s.db.QueryRow(ctx,
		`UPDATE refresh_tokens SET last_used_at = NOW() WHERE token_hash = $1 RETURNING teacher_id`,
		tokenHash,
	).Scan(&teacherID)
	if err != nil {
		return "", ErrInvalidCredentials
	}

	var mobile, schoolName string
	err = s.db.QueryRow(ctx,
		`SELECT mobile, school_name FROM teachers WHERE id = $1`,
		teacherID,
	).Scan(&mobile, &schoolName)
	if err != nil {
		return "", ErrInvalidCredentials
	}

	accessToken, err = s.jwtSvc.GenerateAccessToken(teacherID, mobile, schoolName)
	if err != nil {
		return "", fmt.Errorf("generate access token: %w", err)
	}
	return accessToken, nil
}

// Logout deletes the current device's refresh token.
// Other devices' refresh tokens are untouched (AUTH-12: multi-device).
func (s *Service) Logout(ctx context.Context, rawRefreshToken string) error {
	if rawRefreshToken == "" {
		return nil
	}
	tokenHash := jwtpkg.HashToken(rawRefreshToken)
	_, err := s.db.Exec(ctx,
		`DELETE FROM refresh_tokens WHERE token_hash = $1`,
		tokenHash,
	)
	return err
}
