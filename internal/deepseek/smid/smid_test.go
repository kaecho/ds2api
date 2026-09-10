package smid

import "testing"

func TestIsValid(t *testing.T) {
	if IsValid("") {
		t.Fatal("empty should be invalid")
	}
	if IsValid("7d2a1c4e-9b50-4a6f-8e21-3c9f0a17b6d2") {
		t.Fatal("uuid should be invalid")
	}
	if IsValid("Bshort") {
		t.Fatal("too-short B prefix should be invalid")
	}
	ok := "BNlM4B07tkAd9/BDOrew4lrBnS6RPQZfDZczOl8c94iSKWdOj9pN3BGLJEkfd0a0FKg7oO0/65VIvtFLf4O8X1A=="
	if !IsValid(ok) {
		t.Fatalf("expected valid smid: %s", ok)
	}
}
