package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dockfin/dockfin/internal/crypto"
	"github.com/dockfin/dockfin/internal/mailer"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
)

// handleLogin2FA completes a password login that required a second factor.
func (a *API) handleLogin2FA(w http.ResponseWriter, r *http.Request) {
	ip := a.rateLimitIP(r)
	if !globalLoginLimiter.allow(ip) {
		writeError(w, http.StatusTooManyRequests, "too many login attempts")
		return
	}
	var body struct {
		ChallengeID  string `json:"challenge_id"`
		Code         string `json:"code"`
		RecoveryCode string `json:"recovery_code"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	challengeID, err := uuid.Parse(strings.TrimSpace(body.ChallengeID))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid challenge_id")
		return
	}
	challenge, err := a.Store.GetAuthChallenge(r.Context(), challengeID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired challenge")
		return
	}
	if challenge.Kind != "totp" {
		writeError(w, http.StatusUnauthorized, "invalid or expired challenge")
		return
	}
	user, err := a.Store.GetUserByID(r.Context(), challenge.UserID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired challenge")
		return
	}

	ok := false
	code := strings.TrimSpace(body.Code)
	recovery := strings.TrimSpace(body.RecoveryCode)
	if code != "" {
		secret, enabled, err := a.Store.GetUserTOTPSecret(r.Context(), user.ID)
		if err == nil && enabled && secret != "" {
			ok = totp.Validate(code, secret)
		}
	} else if recovery != "" {
		consumed, err := a.Store.ConsumeRecoveryCode(r.Context(), user.ID, recovery)
		ok = err == nil && consumed
	}
	if !ok {
		globalLoginLimiter.fail(ip)
		writeError(w, http.StatusUnauthorized, "invalid code")
		return
	}
	// One-time use: consume the login challenge regardless of code vs recovery code path.
	_, _ = a.Store.ConsumeAuthChallenge(r.Context(), challengeID)
	globalLoginLimiter.success(ip)

	token, teams, err := a.issueSession(w, r, user)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "team": teams[0], "teams": teams, "token": token})
}

// handleTOTPSetup generates (and stores, pending) a new TOTP secret for the current user.
func (a *API) handleTOTPSetup(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if enabled, err := a.Store.UserHasTOTP(r.Context(), user.ID); err != nil {
		mapStoreErr(w, err)
		return
	} else if enabled {
		writeError(w, http.StatusConflict, "2fa already enabled; disable first")
		return
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "Dockfin",
		AccountName: user.Email,
	})
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if err := a.Store.SetUserTOTPSecret(r.Context(), user.ID, key.Secret()); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"secret":      key.Secret(),
		"otpauth_url": key.URL(),
	})
}

// handleTOTPEnable verifies the setup code and turns on TOTP, returning one-time recovery codes.
func (a *API) handleTOTPEnable(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	var body struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(r, &body); err != nil || strings.TrimSpace(body.Code) == "" {
		writeError(w, http.StatusBadRequest, "code required")
		return
	}
	secret, _, err := a.Store.GetUserTOTPSecret(r.Context(), user.ID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if secret == "" {
		writeError(w, http.StatusBadRequest, "run 2fa setup first")
		return
	}
	if !totp.Validate(strings.TrimSpace(body.Code), secret) {
		writeError(w, http.StatusUnauthorized, "invalid code")
		return
	}
	codes, err := generateRecoveryCodes(8)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if err := a.Store.EnableUserTOTP(r.Context(), user.ID, codes); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "enabled", "recovery_codes": codes})
}

// handleTOTPDisable turns off TOTP after verifying the user's password or a valid TOTP code.
func (a *API) handleTOTPDisable(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	var body struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	verified := false
	if body.Password != "" {
		hash, err := a.Store.GetUserPasswordHash(r.Context(), user.ID)
		if err == nil && hash != "" && crypto.VerifyPassword(hash, body.Password) {
			verified = true
		}
	}
	if !verified && strings.TrimSpace(body.Code) != "" {
		secret, enabled, err := a.Store.GetUserTOTPSecret(r.Context(), user.ID)
		if err == nil && enabled && secret != "" && totp.Validate(strings.TrimSpace(body.Code), secret) {
			verified = true
		}
	}
	if !verified {
		writeError(w, http.StatusUnauthorized, "password or code required")
		return
	}
	if err := a.Store.DisableUserTOTP(r.Context(), user.ID); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
}

// handleTOTPStatus reports whether the current user has TOTP enabled.
func (a *API) handleTOTPStatus(w http.ResponseWriter, r *http.Request) {
	enabled, err := a.Store.UserHasTOTP(r.Context(), currentUser(r).ID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": enabled})
}

func generateRecoveryCodes(n int) ([]string, error) {
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		t, err := crypto.RandomToken(5)
		if err != nil {
			return nil, err
		}
		out = append(out, strings.ToUpper(strings.TrimRight(t, "=")))
	}
	return out, nil
}

// handleForgotPassword always returns a generic 200 to avoid leaking account existence.
func (a *API) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	_ = decodeJSON(r, &body)
	defer writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})

	email := strings.TrimSpace(body.Email)
	if email == "" {
		return
	}
	user, _, err := a.Store.GetUserByEmail(r.Context(), email)
	if err != nil {
		return
	}
	inst, err := a.Store.GetInstanceSettings(r.Context())
	if err != nil || (!inst.SMTPEnabled && !inst.ResendEnabled) {
		return
	}
	if inst.SMTPFromAddress == "" {
		return
	}
	token, err := crypto.RandomToken(32)
	if err != nil {
		return
	}
	if err := a.Store.CreatePasswordResetToken(r.Context(), user.Email, token, time.Hour); err != nil {
		if a.Logger != nil {
			a.Logger.Warn("create password reset token failed", "error", err.Error())
		}
		return
	}
	base := strings.TrimRight(strings.TrimSpace(inst.PublicURL), "/")
	if base == "" && a.Cfg != nil {
		base = strings.TrimRight(strings.TrimSpace(a.Cfg.PublicURL), "/")
	}
	if base == "" {
		return
	}
	link := base + "/reset-password?token=" + token
	msg := mailer.Message{
		FromName: inst.SMTPFromName,
		From:     inst.SMTPFromAddress,
		To:       []string{user.Email},
		Subject:  "Reset your Dockfin password",
		Body:     fmt.Sprintf("Click the link below to reset your Dockfin password:\n\n%s\n\nThis link expires in 1 hour. If you did not request this, you can ignore this email.", link),
	}
	smtpUser, smtpPass, resendKey, err := a.Store.SMTPMaterial(r.Context())
	if err != nil {
		return
	}
	var sendErr error
	if inst.ResendEnabled {
		if resendKey == "" {
			return
		}
		sendErr = mailer.SendResend(r.Context(), resendKey, msg)
	} else {
		if inst.SMTPHost == "" {
			return
		}
		timeout := 30
		if inst.SMTPTimeout != nil {
			timeout = *inst.SMTPTimeout
		}
		sendErr = mailer.SendSMTP(r.Context(), mailer.SMTPConfig{
			Host:       inst.SMTPHost,
			Port:       inst.SMTPPort,
			Encryption: inst.SMTPEncryption,
			Username:   smtpUser,
			Password:   smtpPass,
			TimeoutSec: timeout,
		}, msg)
	}
	if sendErr != nil && a.Logger != nil {
		a.Logger.Warn("send password reset email failed", "error", sendErr.Error())
	}
}

// handleResetPassword completes a password reset given a valid token.
func (a *API) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Token == "" || len(body.Password) < 8 {
		writeError(w, http.StatusBadRequest, "token and password (min 8) required")
		return
	}
	email, err := a.Store.ConsumePasswordResetToken(r.Context(), body.Token)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid or expired token")
		return
	}
	user, _, err := a.Store.GetUserByEmail(r.Context(), email)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	hash, err := crypto.HashPassword(body.Password)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if err := a.Store.UpdatePassword(r.Context(), user.ID, hash); err != nil {
		mapStoreErr(w, err)
		return
	}
	_ = a.Store.DeleteSessionsForUser(r.Context(), user.ID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
