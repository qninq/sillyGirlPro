package core

import (
	"fmt"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestAdminNormalUserLifecycle(t *testing.T) {
	username := fmt.Sprintf("admin_test_%d", time.Now().UnixNano())
	_, err := createNormalUser(username, "initial-password", "初始昵称")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deleteNormalUser(username) }()

	bindings, err := replaceNormalUserBindings(username, "12345678", "-123456789")
	if err != nil {
		t.Fatal(err)
	}
	if bindings.QQ != "12345678" || bindings.Telegram != "-123456789" {
		t.Fatalf("unexpected bindings: %+v", bindings)
	}

	disabled := true
	updated, updatedBindings, err := updateNormalUserByAdmin(adminNormalUserPayload{
		Username: username,
		Password: "updated-password",
		Nickname: "更新昵称",
		Disabled: &disabled,
		QQ:       "87654321",
		Telegram: "987654321",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Nickname != "更新昵称" || !updated.Disabled {
		t.Fatalf("unexpected updated user: %+v", updated)
	}
	if bcrypt.CompareHashAndPassword([]byte(updated.PasswordHash), []byte("updated-password")) != nil {
		t.Fatal("password was not updated")
	}
	if updatedBindings.QQ != "87654321" {
		t.Fatalf("unexpected updated bindings: %+v", updatedBindings)
	}

	if err := deleteNormalUser(username); err != nil {
		t.Fatal(err)
	}
	if _, err := loadNormalUser(username); err == nil {
		t.Fatal("deleted user can still be loaded")
	}
	if raw := userBucket.GetString(normalUserBindingsStorageKey(username)); raw != "" {
		t.Fatalf("bindings were not deleted: %s", raw)
	}
}

func TestNormalizedReplacementBindingsValidation(t *testing.T) {
	if _, err := normalizedReplacementBindings("abc", ""); err == nil {
		t.Fatal("invalid QQ should be rejected")
	}
	if _, err := normalizedReplacementBindings("123456", "telegram-user"); err == nil {
		t.Fatal("invalid Telegram ID should be rejected")
	}
}
