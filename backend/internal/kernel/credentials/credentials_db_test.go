package credentials

import (
	"context"
	"os"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/security"

	"github.com/jackc/pgx/v5/pgxpool"
)

// What a credential store has to be, asked of a real database: sealed on the
// way in, resolvable on the way out, and never readable through the thing the
// console looks at.

func pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("CONTROLPLANE_TEST_DATABASE_URL")
	if url == "" {
		url = os.Getenv("DATABASE_URL")
	}
	if url == "" {
		t.Skip("neither CONTROLPLANE_TEST_DATABASE_URL nor DATABASE_URL is set")
	}
	p, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(p.Close)
	if err := p.Ping(context.Background()); err != nil {
		t.Skipf("database unreachable: %v", err)
	}
	return p
}

func TestAStoredCredentialIsSealedAndResolves(t *testing.T) {
	t.Setenv(security.EncryptionKeyEnv, "a-passphrase-for-this-test")
	security.ResetEncryptionKeyForTest()
	t.Cleanup(security.ResetEncryptionKeyForTest)

	db := pool(t)
	ctx := context.Background()
	store := NewStore(db)

	// In a transaction that is rolled back: the test database is shared, and a
	// credential left behind would be one the next run's environment fallback
	// silently loses to.
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const secret = "sk-test-0123456789-abcd"
	if err := store.Set(ctx, tx, GeminiAPIKey, secret, "00000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatalf("set: %v", err)
	}

	// What landed in the column is not the secret. This is the whole promise of
	// the table, so it is asserted against the bytes rather than inferred.
	var ciphertext []byte
	var storedHint string
	if err := tx.QueryRow(ctx,
		`SELECT ciphertext, hint FROM operator.platform_credentials WHERE name = $1`,
		GeminiAPIKey).Scan(&ciphertext, &storedHint); err != nil {
		t.Fatalf("read the row: %v", err)
	}
	if string(ciphertext) == secret {
		t.Fatal("the credential was stored in the clear")
	}
	if storedHint != "abcd" {
		t.Fatalf("the hint is %q, not the last four characters", storedHint)
	}

	plaintext, err := security.Open(ciphertext)
	if err != nil || string(plaintext) != secret {
		t.Fatalf("the stored credential does not decrypt to what was set: %q, %v", plaintext, err)
	}
}

func TestGetPrefersTheStoredValueAndFallsBackToTheEnvironment(t *testing.T) {
	t.Setenv(security.EncryptionKeyEnv, "a-passphrase-for-this-test")
	t.Setenv("GEMINI_API_KEY", "from-the-environment")
	security.ResetEncryptionKeyForTest()
	t.Cleanup(security.ResetEncryptionKeyForTest)

	store := NewStore(pool(t))
	previous := Default
	UseStore(store)
	t.Cleanup(func() { UseStore(previous) })

	// Nothing loaded yet: the environment is the answer, which is what a
	// deployment that has never opened the console must keep seeing.
	if got := Get(GeminiAPIKey); got != "from-the-environment" {
		t.Fatalf("without a stored value Get answered %q", got)
	}

	// A value in the cache wins, because somebody chose it more recently.
	store.mu.Lock()
	store.values[GeminiAPIKey] = "from-the-console"
	store.mu.Unlock()
	if got := Get(GeminiAPIKey); got != "from-the-console" {
		t.Fatalf("with a stored value Get answered %q", got)
	}

	// And a name nothing declared is never an answer: a typo at a call site
	// must not read as a credential somebody forgot to set.
	if got := Get("nothing.declared_this"); got != "" {
		t.Fatalf("an unregistered name answered %q", got)
	}
}

func TestListNeverCarriesAValue(t *testing.T) {
	t.Setenv(security.EncryptionKeyEnv, "a-passphrase-for-this-test")
	t.Setenv("GEMINI_API_KEY", "from-the-environment")
	security.ResetEncryptionKeyForTest()
	t.Cleanup(security.ResetEncryptionKeyForTest)

	store := NewStore(pool(t))
	statuses, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(statuses) != len(registry) {
		t.Fatalf("list returned %d of %d registered credentials", len(statuses), len(registry))
	}
	for _, status := range statuses {
		if status.Name == GeminiAPIKey && status.Source != "environment" {
			t.Fatalf("the source of a variable-backed credential is %q", status.Source)
		}
		// Status has no field that could hold one, which is the point of the
		// type; this is the check that notices the day somebody adds one.
		if status.Hint == "from-the-environment" {
			t.Fatal("the hint carried the environment's value")
		}
	}
}
