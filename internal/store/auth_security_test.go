package store_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dockfin/dockfin/internal/crypto"
	"github.com/dockfin/dockfin/internal/db"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

func mustRandomSuffix(t *testing.T) string {
	t.Helper()
	s, err := crypto.RandomToken(6)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func newTestStore(t *testing.T) (*store.Store, context.Context) {
	t.Helper()
	dsn := os.Getenv("DOCKFIN_DATABASE_URL")
	if dsn == "" {
		t.Skip("DOCKFIN_DATABASE_URL not set")
	}
	box, err := crypto.NewBox("test-master-key-for-integration-32b!")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(dsn); err != nil {
		t.Logf("migrate warning: %v", err)
	}
	return store.New(pool, box), ctx
}

func TestCreateTeam(t *testing.T) {
	st, ctx := newTestStore(t)

	email := "team-owner-" + mustRandomSuffix(t) + "@example.com"
	hash, err := crypto.HashPassword("supersecret1")
	if err != nil {
		t.Fatal(err)
	}
	user, _, err := st.CreateUserWithPersonalTeam(ctx, email, "Team Owner", hash)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	team, err := st.CreateTeam(ctx, user.ID, "Extra Team", "a non-personal team")
	if err != nil {
		t.Fatalf("create team: %v", err)
	}
	if team.Personal {
		t.Fatal("expected non-personal team")
	}
	if team.Role != "owner" {
		t.Fatalf("expected owner role, got %q", team.Role)
	}

	role, err := st.UserRoleOnTeam(ctx, user.ID, team.ID)
	if err != nil || role != "owner" {
		t.Fatalf("expected owner membership, got role=%q err=%v", role, err)
	}

	if _, err := st.CreateTeam(ctx, user.ID, "", "no name"); err == nil {
		t.Fatal("expected error for empty team name")
	}
}

func TestAuthChallengeCreateAndConsume(t *testing.T) {
	st, ctx := newTestStore(t)

	email := "challenge-user-" + mustRandomSuffix(t) + "@example.com"
	hash, err := crypto.HashPassword("supersecret1")
	if err != nil {
		t.Fatal(err)
	}
	user, _, err := st.CreateUserWithPersonalTeam(ctx, email, "Challenge User", hash)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	challenge, err := st.CreateAuthChallenge(ctx, user.ID, "totp", 5*time.Minute)
	if err != nil {
		t.Fatalf("create challenge: %v", err)
	}
	if challenge.UserID != user.ID || challenge.Kind != "totp" {
		t.Fatalf("unexpected challenge: %+v", challenge)
	}

	// First consume succeeds and returns the same challenge.
	consumed, err := st.ConsumeAuthChallenge(ctx, challenge.ID)
	if err != nil {
		t.Fatalf("consume challenge: %v", err)
	}
	if consumed.ID != challenge.ID {
		t.Fatalf("unexpected consumed challenge id: %v", consumed.ID)
	}

	// Second consume must fail (one-time use).
	if _, err := st.ConsumeAuthChallenge(ctx, challenge.ID); err != store.ErrNotFound {
		t.Fatalf("expected ErrNotFound on second consume, got %v", err)
	}

	// Expired challenge should not be consumable.
	expired, err := st.CreateAuthChallenge(ctx, user.ID, "totp", -1*time.Minute)
	if err != nil {
		t.Fatalf("create expired challenge: %v", err)
	}
	if _, err := st.ConsumeAuthChallenge(ctx, expired.ID); err != store.ErrNotFound {
		t.Fatalf("expected ErrNotFound for expired challenge, got %v", err)
	}
}

func TestTOTPSetupEnableDisableAndRecoveryCodes(t *testing.T) {
	st, ctx := newTestStore(t)

	email := "totp-user-" + mustRandomSuffix(t) + "@example.com"
	hash, err := crypto.HashPassword("supersecret1")
	if err != nil {
		t.Fatal(err)
	}
	user, _, err := st.CreateUserWithPersonalTeam(ctx, email, "TOTP User", hash)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	if has, err := st.UserHasTOTP(ctx, user.ID); err != nil || has {
		t.Fatalf("expected TOTP disabled by default: has=%v err=%v", has, err)
	}

	if err := st.SetUserTOTPSecret(ctx, user.ID, "JBSWY3DPEHPK3PXP"); err != nil {
		t.Fatalf("set totp secret: %v", err)
	}
	secret, enabled, err := st.GetUserTOTPSecret(ctx, user.ID)
	if err != nil || secret != "JBSWY3DPEHPK3PXP" || enabled {
		t.Fatalf("unexpected pending totp state: secret=%q enabled=%v err=%v", secret, enabled, err)
	}

	codes := []string{"AAAA1111", "BBBB2222"}
	if err := st.EnableUserTOTP(ctx, user.ID, codes); err != nil {
		t.Fatalf("enable totp: %v", err)
	}
	if has, err := st.UserHasTOTP(ctx, user.ID); err != nil || !has {
		t.Fatalf("expected TOTP enabled: has=%v err=%v", has, err)
	}
	// Re-setup while enabled must fail (must not silently disable 2FA).
	if err := st.SetUserTOTPSecret(ctx, user.ID, "NEWTOTPSECRET1234"); err == nil {
		t.Fatal("expected SetUserTOTPSecret to fail while 2FA enabled")
	}

	ok, err := st.ConsumeRecoveryCode(ctx, user.ID, "aaaa1111")
	if err != nil || !ok {
		t.Fatalf("expected recovery code consumed: ok=%v err=%v", ok, err)
	}
	// Reusing the same code must fail.
	ok, err = st.ConsumeRecoveryCode(ctx, user.ID, "aaaa1111")
	if err != nil || ok {
		t.Fatalf("expected recovery code reuse to fail: ok=%v err=%v", ok, err)
	}
	// The other code is still valid.
	ok, err = st.ConsumeRecoveryCode(ctx, user.ID, "BBBB2222")
	if err != nil || !ok {
		t.Fatalf("expected second recovery code consumed: ok=%v err=%v", ok, err)
	}

	if err := st.DisableUserTOTP(ctx, user.ID); err != nil {
		t.Fatalf("disable totp: %v", err)
	}
	if has, err := st.UserHasTOTP(ctx, user.ID); err != nil || has {
		t.Fatalf("expected TOTP disabled after DisableUserTOTP: has=%v err=%v", has, err)
	}
}

func TestPasswordResetTokenLifecycle(t *testing.T) {
	st, ctx := newTestStore(t)

	email := "reset-user-" + mustRandomSuffix(t) + "@example.com"
	hash, err := crypto.HashPassword("supersecret1")
	if err != nil {
		t.Fatal(err)
	}
	user, _, err := st.CreateUserWithPersonalTeam(ctx, email, "Reset User", hash)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	token, err := crypto.RandomToken(16)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreatePasswordResetToken(ctx, user.Email, token, time.Hour); err != nil {
		t.Fatalf("create reset token: %v", err)
	}

	gotEmail, err := st.ConsumePasswordResetToken(ctx, token)
	if err != nil {
		t.Fatalf("consume reset token: %v", err)
	}
	if gotEmail == "" {
		t.Fatal("expected non-empty email")
	}

	// Token is single-use.
	if _, err := st.ConsumePasswordResetToken(ctx, token); err != store.ErrNotFound {
		t.Fatalf("expected ErrNotFound on reuse, got %v", err)
	}

	newHash, err := crypto.HashPassword("brandnewpassword1")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpdatePassword(ctx, user.ID, newHash); err != nil {
		t.Fatalf("update password: %v", err)
	}
	_, storedHash, err := st.GetUserByEmail(ctx, user.Email)
	if err != nil || !crypto.VerifyPassword(storedHash, "brandnewpassword1") {
		t.Fatalf("password update did not take effect: err=%v", err)
	}
}
