package vinkius_test

import (
	"testing"

	"github.com/VinkiusForDevs/ai-connect-sdk-go/vinkius"
)

func TestNewRejectsInvalidAppID(t *testing.T) {
	if _, err := vinkius.New("wrong", "vk_app_sk_x"); !vinkius.IsConfigError(err) {
		t.Fatalf("bad app id should be a config error: %v", err)
	}
}

func TestNewRejectsSecretPassedAsAppID(t *testing.T) {
	if _, err := vinkius.New("vk_app_sk_secret", "vk_app_sk_y"); !vinkius.IsConfigError(err) {
		t.Fatalf("a secret key passed as the app id should be rejected: %v", err)
	}
}

func TestNewRejectsInvalidKey(t *testing.T) {
	if _, err := vinkius.New("vk_app_x", "wrong"); !vinkius.IsConfigError(err) {
		t.Fatalf("bad key should be a config error: %v", err)
	}
}

func TestUserRejectsInternalID(t *testing.T) {
	v, err := vinkius.New("vk_app_x", "vk_app_sk_y")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := v.User("vk_app_user_123"); !vinkius.IsConfigError(err) {
		t.Fatalf("a Vinkius internal id should be rejected: %v", err)
	}
}

func TestUserRejectsUnsafeExternalID(t *testing.T) {
	v, err := vinkius.New("vk_app_x", "vk_app_sk_y")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := v.User("bad id"); !vinkius.IsConfigError(err) {
		t.Fatalf("an unsafe external id should be rejected: %v", err)
	}
}

func TestNewWiresClients(t *testing.T) {
	v, err := vinkius.New("vk_app_x", "vk_app_sk_y")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer v.Close()
	if v.Catalog == nil || v.Users == nil {
		t.Fatalf("clients not wired: catalog=%v users=%v", v.Catalog, v.Users)
	}
}
