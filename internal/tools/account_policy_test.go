package tools

import "testing"

func TestLoadAccountPolicyMutuallyExclusive(t *testing.T) {
	t.Setenv(allowedAccountsEnvVar, "work")
	t.Setenv(blockedAccountsEnvVar, "private")

	_, err := LoadAccountPolicy()
	if err == nil {
		t.Fatalf("expected mutual exclusivity error, got nil")
	}
}

func TestEnforceAccountAccessAllowedList(t *testing.T) {
	t.Setenv(allowedAccountsEnvVar, "work,work@example.com")
	t.Setenv(blockedAccountsEnvVar, "")

	if err := enforceAccountAccess("Work"); err != nil {
		t.Fatalf("expected Work to be allowed, got error: %v", err)
	}

	if err := enforceAccountAccess("Personal"); err == nil {
		t.Fatalf("expected Personal to be rejected")
	}
}

func TestEnforceAccountAccessBlockedList(t *testing.T) {
	t.Setenv(allowedAccountsEnvVar, "")
	t.Setenv(blockedAccountsEnvVar, "personal,private@example.com")

	if err := enforceAccountAccess("Personal"); err == nil {
		t.Fatalf("expected Personal to be blocked")
	}

	if err := enforceAccountAccess("Work"); err != nil {
		t.Fatalf("expected Work to be allowed, got error: %v", err)
	}
}

func TestEnforceSenderAccessEmailInBrackets(t *testing.T) {
	t.Setenv(allowedAccountsEnvVar, "")
	t.Setenv(blockedAccountsEnvVar, "private@example.com")

	err := enforceSenderAccess("Private Person <private@example.com>")
	if err == nil {
		t.Fatalf("expected sender to be blocked")
	}
}

func TestFilterMessagesByAccountField(t *testing.T) {
	t.Setenv(allowedAccountsEnvVar, "work")
	t.Setenv(blockedAccountsEnvVar, "")

	input := map[string]any{
		"messages": []any{
			map[string]any{"account": "Work", "id": 1},
			map[string]any{"account": "Personal", "id": 2},
		},
		"count": 2,
	}

	filtered, err := filterMessagesByAccountField(input, "account")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := filtered.(map[string]any)
	messages := out["messages"].([]any)
	if len(messages) != 1 {
		t.Fatalf("expected 1 message after filtering, got %d", len(messages))
	}
}

func TestFilterListAccountsData(t *testing.T) {
	t.Setenv(allowedAccountsEnvVar, "work@example.com")
	t.Setenv(blockedAccountsEnvVar, "")

	input := map[string]any{
		"accounts": []any{
			map[string]any{"name": "Personal", "emailAddresses": []any{"personal@example.com"}},
			map[string]any{"name": "Work", "emailAddresses": []any{"work@example.com"}},
		},
		"count": 2,
	}

	filtered, err := filterListAccountsData(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := filtered.(map[string]any)
	accounts := out["accounts"].([]any)
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account after filtering, got %d", len(accounts))
	}
}
