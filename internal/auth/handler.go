package auth

import (
	"encoding/json"
	"errors"
	"net/http"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

type sendOTPRequest struct {
	Name       string `json:"name"`
	Mobile     string `json:"mobile"`
	SchoolName string `json:"school_name"`
	Password   string `json:"password"`
}

type sendOTPResponse struct {
	ReqID string `json:"req_id"`
}

func (h *Handler) SendRegistrationOTP(w http.ResponseWriter, r *http.Request) {
	var req sendOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}

	reqID, err := h.svc.SendRegistrationOTP(r.Context(), req.Name, req.Mobile, req.SchoolName, req.Password)
	if err != nil {
		var ve *ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusBadRequest, "validation_error", ve.Message)
			return
		}
		if errors.Is(err, ErrDuplicateMobile) {
			writeError(w, http.StatusConflict, "duplicate_mobile", "This mobile number is already registered. Please login.")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to send OTP")
		return
	}

	writeJSON(w, http.StatusOK, sendOTPResponse{ReqID: reqID})
}

type verifyOTPRequest struct {
	ReqID      string `json:"req_id"`
	OTP        string `json:"otp"`
	Name       string `json:"name"`
	Mobile     string `json:"mobile"`
	SchoolName string `json:"school_name"`
	Password   string `json:"password"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TeacherID    string `json:"teacher_id,omitempty"`
}

func (h *Handler) VerifyRegistrationOTP(w http.ResponseWriter, r *http.Request) {
	var req verifyOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	if req.ReqID == "" || req.OTP == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "req_id and otp are required")
		return
	}

	accessToken, refreshToken, err := h.svc.VerifyRegistrationOTP(
		r.Context(), req.ReqID, req.OTP, req.Name, req.Mobile, req.SchoolName, req.Password,
	)
	if err != nil {
		switch {
		case errors.Is(err, ErrSessionNotFound):
			writeError(w, http.StatusBadRequest, "session_not_found", "OTP session not found or expired")
		case errors.Is(err, ErrOTPExpired):
			writeError(w, http.StatusBadRequest, "otp_expired", "OTP has expired. Please request a new one.")
		case errors.Is(err, ErrOTPLocked):
			writeError(w, http.StatusForbidden, "otp_locked", "Too many incorrect attempts. Please request a new OTP.")
		case errors.Is(err, ErrOTPInvalid):
			writeError(w, http.StatusBadRequest, "otp_invalid", "Incorrect OTP. Please try again.")
		case errors.Is(err, ErrSessionAlreadyUsed):
			writeError(w, http.StatusBadRequest, "session_used", "This OTP session has already been used.")
		default:
			writeError(w, http.StatusInternalServerError, "internal_error", "Registration failed")
		}
		return
	}

	writeJSON(w, http.StatusCreated, tokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

type registerRequest struct {
	Name       string `json:"name"`
	Mobile     string `json:"mobile"`
	SchoolName string `json:"school_name"`
	Password   string `json:"password"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}

	accessToken, refreshToken, teacherID, err := h.svc.RegisterDirect(r.Context(), req.Name, req.Mobile, req.SchoolName, req.Password)
	if err != nil {
		var ve *ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusBadRequest, "validation_error", ve.Message)
			return
		}
		if errors.Is(err, ErrDuplicateMobile) {
			writeError(w, http.StatusConflict, "duplicate_mobile", "This mobile number is already registered. Please login.")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Registration failed")
		return
	}

	writeJSON(w, http.StatusCreated, tokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TeacherID:    teacherID,
	})
}

type retryOTPRequest struct {
	ReqID        string `json:"req_id"`
	RetryChannel int    `json:"retry_channel"`
}

func (h *Handler) RetryOTP(w http.ResponseWriter, r *http.Request) {
	var req retryOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	if req.ReqID == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "req_id is required")
		return
	}
	channel := req.RetryChannel
	if channel == 0 {
		channel = 11 // default to SMS
	}

	if err := h.svc.RetryOTP(r.Context(), req.ReqID, channel); err != nil {
		if errors.Is(err, ErrSessionNotFound) || errors.Is(err, ErrSessionExpiredOrUsed) {
			writeError(w, http.StatusBadRequest, "session_invalid", "OTP session not found or expired")
			return
		}
		if errors.Is(err, ErrRetryTooSoon) {
			writeError(w, http.StatusTooManyRequests, "retry_too_soon", "Please wait before requesting another OTP")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to retry OTP")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

type loginRequest struct {
	Mobile     string `json:"mobile"`
	Password   string `json:"password"`
	DeviceHint string `json:"device_hint"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}

	accessToken, refreshToken, err := h.svc.Login(r.Context(), req.Mobile, req.Password, req.DeviceHint)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid mobile number or password")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Login failed")
		return
	}

	writeJSON(w, http.StatusOK, tokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type refreshResponse struct {
	AccessToken string `json:"access_token"`
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}

	accessToken, err := h.svc.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_token", "Refresh token is invalid or has been revoked")
		return
	}

	writeJSON(w, http.StatusOK, refreshResponse{AccessToken: accessToken})
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var req logoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}

	if err := h.svc.Logout(r.Context(), req.RefreshToken); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Logout failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}
