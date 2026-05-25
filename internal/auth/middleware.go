package auth

import (
	"context"
	"net/http"
	"strings"

	jwtpkg "school-management/backend/pkg/jwt"
)

type contextKey string

const (
	ContextKeyTeacherID  contextKey = "teacher_id"
	ContextKeyMobile     contextKey = "mobile"
	ContextKeySchoolName contextKey = "school_name"
)

// JWTMiddleware validates the Bearer token in the Authorization header.
// On success, sets teacher_id, mobile, school_name in the request context.
// On failure, returns 401 — never 403, to avoid leaking route existence.
func JWTMiddleware(jwtSvc *jwtpkg.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeError(w, http.StatusUnauthorized, "missing_token", "Authorization header is required")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				writeError(w, http.StatusUnauthorized, "invalid_token_format", "Authorization header must be: Bearer <token>")
				return
			}

			claims, err := jwtSvc.ParseAccessToken(parts[1])
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid_token", "Token is invalid or expired")
				return
			}

			ctx := context.WithValue(r.Context(), ContextKeyTeacherID, claims.TeacherID)
			ctx = context.WithValue(ctx, ContextKeyMobile, claims.Mobile)
			ctx = context.WithValue(ctx, ContextKeySchoolName, claims.SchoolName)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// TeacherIDFromContext extracts the authenticated teacher ID from the request context.
func TeacherIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(ContextKeyTeacherID).(string)
	return id
}
