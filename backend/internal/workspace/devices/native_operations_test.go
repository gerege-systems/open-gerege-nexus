package devices

import (
	"encoding/base64"
	"testing"
)

func TestPushTokenEncryptionDoesNotPersistPlaintext(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	t.Setenv("PUSH_TOKEN_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(key))
	got, err := encryptPushToken("fcm-secret-token-value")
	if err != nil {
		t.Fatal(err)
	}
	if got == "fcm-secret-token-value" {
		t.Fatal("token was not encrypted")
	}
	if _, err = base64.StdEncoding.DecodeString(got); err != nil {
		t.Fatal("ciphertext is not transport-safe base64")
	}
}
func TestPushTokenEncryptionRequiresKey(t *testing.T) {
	t.Setenv("PUSH_TOKEN_ENCRYPTION_KEY", "")
	if _, err := encryptPushToken("token"); err == nil {
		t.Fatal("missing encryption key accepted")
	}
}
