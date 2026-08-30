/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package staffpin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/security"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Кассны PIN таах оролдлого тоологдож, хэт олон болмогц тэр касс хаагдана.
//
// Энэ нь өгөгдлийн сангаас өөр газар шалгагдахгүй: тоолол ба түгжээ хоёул нэг
// UPDATE-ийн дотор, `staff_pin_failures + 1 >= 5` гэсэн нөхцөлөөр шийдэгддэг.
// Тэр багануудыг 00040 нэмсэн боловч 00105 хүртэл тэднийг нэмэгдүүлдэг код
// байгаагүй тул түгжээ нь бичигдсэн атлаа хэзээ ч биелдэггүй байв.

func pinPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// fixture builds an organisation with one till and one member holding a PIN.
func fixture(t *testing.T, pool *pgxpool.Pool, pin string) (tenantID, deviceID string) {
	t.Helper()
	ctx := context.Background()
	stamp := time.Now().UnixNano()

	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.tenants (slug, name) VALUES ($1, $1) RETURNING id::text`,
		fmt.Sprintf("pin-%d", stamp)).Scan(&tenantID); err != nil {
		t.Fatalf("organisation: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM registry.tenants WHERE id = $1::uuid`, tenantID)
	})

	var userID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.users (email, password_hash, name) VALUES ($1, 'x', 'Till Staff') RETURNING id::text`,
		fmt.Sprintf("pin-%d@staff.test", stamp)).Scan(&userID); err != nil {
		t.Fatalf("person: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM registry.users WHERE id = $1::uuid`, userID)
	})

	var membershipID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO workspace.memberships (tenant_id, user_id) VALUES ($1::uuid, $2::uuid) RETURNING id::text`,
		tenantID, userID).Scan(&membershipID); err != nil {
		t.Fatalf("membership: %v", err)
	}

	hash, err := security.HashPIN(pin)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspace.staff_pin_credentials (tenant_id, membership_id, pin_hash, active)
		 VALUES ($1::uuid, $2::uuid, $3, true)`, tenantID, membershipID, hash); err != nil {
		t.Fatalf("credential: %v", err)
	}

	if err := pool.QueryRow(ctx,
		`INSERT INTO workspace.devices (tenant_id, name, platform, form_factor, token_hash)
		 VALUES ($1::uuid, 'Till', 'android', 'pos', $2) RETURNING id::text`,
		tenantID, fmt.Sprintf("hash-%d", stamp)).Scan(&deviceID); err != nil {
		t.Fatalf("device: %v", err)
	}
	return tenantID, deviceID
}

func TestATillThatKeepsGuessingIsShut(t *testing.T) {
	pool := pinPool(t)
	ctx := context.Background()
	service := &Service{db: pool}
	tenantID, deviceID := fixture(t, pool, "4821")

	for attempt := 1; attempt <= staffPINMaxFailures; attempt++ {
		if _, err := service.VerifyOnDevice(ctx, tenantID, deviceID, "0000"); !errors.Is(err, ErrStaffCredentialRejected) {
			t.Fatalf("attempt %d answered %v", attempt, err)
		}
	}

	var failures int
	var lockedUntil *time.Time
	if err := pool.QueryRow(ctx,
		`SELECT staff_pin_failures, staff_pin_locked_until FROM workspace.devices WHERE id = $1::uuid`,
		deviceID).Scan(&failures, &lockedUntil); err != nil {
		t.Fatal(err)
	}
	if failures != staffPINMaxFailures {
		t.Errorf("the till counted %d failures, want %d", failures, staffPINMaxFailures)
	}
	if lockedUntil == nil || !lockedUntil.After(time.Now()) {
		t.Fatalf("the till is not shut after %d wrong PINs: %v", staffPINMaxFailures, lockedUntil)
	}

	// Түгжигдсэн касс ЗӨВ PIN-ийг ч хүлээж авахгүй — тэгэхгүй бол таагаад
	// байгаа хүн зөв утга дээр очиход түгжээ утгагүй болно.
	if _, err := service.VerifyOnDevice(ctx, tenantID, deviceID, "4821"); !errors.Is(err, ErrStaffCredentialRejected) {
		t.Errorf("a shut till accepted a correct PIN: %v", err)
	}
}

// Зөв PIN тоололыг тэглэнэ: өдрийн турш нэг нэгээр буруу дарсан нь хуримтлагдаж
// шөнө дунд кассыг хаах ёсгүй.
func TestACorrectPINClearsTheCount(t *testing.T) {
	pool := pinPool(t)
	ctx := context.Background()
	service := &Service{db: pool}
	tenantID, deviceID := fixture(t, pool, "7391")

	for i := 0; i < staffPINMaxFailures-1; i++ {
		if _, err := service.VerifyOnDevice(ctx, tenantID, deviceID, "0000"); !errors.Is(err, ErrStaffCredentialRejected) {
			t.Fatalf("wrong PIN answered %v", err)
		}
	}
	identity, err := service.VerifyOnDevice(ctx, tenantID, deviceID, "7391")
	if err != nil {
		t.Fatalf("the correct PIN was refused: %v", err)
	}
	if identity.Name != "Till Staff" {
		t.Errorf("the wrong person was identified: %+v", identity)
	}

	var failures int
	if err := pool.QueryRow(ctx,
		`SELECT staff_pin_failures FROM workspace.devices WHERE id = $1::uuid`, deviceID).Scan(&failures); err != nil {
		t.Fatal(err)
	}
	if failures != 0 {
		t.Errorf("the count survived a correct PIN: %d", failures)
	}
}

// PIN-ийн hash нь нууц үгийнхээс хямд параметртэй: касс нь нэргүй ирсэн
// цифрүүдийг байгууллагын credential бүртэй тулгадаг тул нэг товшилт нь
// ажилтны тоогоор үржигдэнэ.
func TestAPINIsHashedAtThePINCost(t *testing.T) {
	hash, err := security.HashPIN("4821")
	if err != nil {
		t.Fatal(err)
	}
	if !security.CheckPasswordHash("4821", hash) {
		t.Fatal("a PIN does not verify against its own hash")
	}
	if security.NeedsRehash(hash) {
		t.Error("a PIN hash is reported as stale, so every tap would rewrite it")
	}
	password, err := security.HashPassword("4821")
	if err != nil {
		t.Fatal(err)
	}
	if hash == password {
		t.Error("the two hashes are identical, so they cannot carry different costs")
	}
}
