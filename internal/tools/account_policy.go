package tools

import (
	"fmt"
	"os"
	"strings"
)

const (
	allowedAccountsEnvVar = "APPLE_MAIL_MCP_ALLOWED_ACCOUNTS"
	blockedAccountsEnvVar = "APPLE_MAIL_MCP_BLOCKED_ACCOUNTS"
)

type accountPolicy struct {
	allowed map[string]struct{}
	blocked map[string]struct{}
}

// LoadAccountPolicy reads and validates account policy environment variables.
func LoadAccountPolicy() (accountPolicy, error) {
	allowed := parsePolicyList(os.Getenv(allowedAccountsEnvVar))
	blocked := parsePolicyList(os.Getenv(blockedAccountsEnvVar))

	if len(allowed) > 0 && len(blocked) > 0 {
		return accountPolicy{}, fmt.Errorf("invalid account policy: %s and %s are mutually exclusive", allowedAccountsEnvVar, blockedAccountsEnvVar)
	}

	return accountPolicy{
		allowed: allowed,
		blocked: blocked,
	}, nil
}

func parsePolicyList(raw string) map[string]struct{} {
	items := map[string]struct{}{}
	for _, part := range strings.Split(raw, ",") {
		v := normalizeIdentity(part)
		if v == "" {
			continue
		}
		items[v] = struct{}{}
	}
	return items
}

func (p accountPolicy) hasRules() bool {
	return len(p.allowed) > 0 || len(p.blocked) > 0
}

func (p accountPolicy) matchBlocked(identity string) bool {
	for _, c := range identityCandidates(identity) {
		if _, ok := p.blocked[c]; ok {
			return true
		}
	}
	return false
}

func (p accountPolicy) matchAllowed(identity string) bool {
	if len(p.allowed) == 0 {
		return true
	}
	for _, c := range identityCandidates(identity) {
		if _, ok := p.allowed[c]; ok {
			return true
		}
	}
	return false
}

func (p accountPolicy) isPermitted(identity string) bool {
	if !p.hasRules() {
		return true
	}

	if !p.matchAllowed(identity) {
		return false
	}

	if p.matchBlocked(identity) {
		return false
	}

	return true
}

func normalizeIdentity(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func identityCandidates(identity string) []string {
	normalized := normalizeIdentity(identity)
	if normalized == "" {
		return nil
	}

	candidates := []string{normalized}

	if start := strings.LastIndex(normalized, "<"); start >= 0 {
		if end := strings.LastIndex(normalized, ">"); end > start+1 {
			email := strings.TrimSpace(normalized[start+1 : end])
			if email != "" && email != normalized {
				candidates = append(candidates, email)
			}
		}
	}

	return candidates
}

func enforceAccountAccess(account string) error {
	policy, err := LoadAccountPolicy()
	if err != nil {
		return err
	}

	if !policy.hasRules() {
		return nil
	}

	if !policy.matchAllowed(account) {
		return fmt.Errorf("account policy violation: account %q is not allowed", account)
	}

	if policy.matchBlocked(account) {
		return fmt.Errorf("account policy violation: account %q is blocked", account)
	}

	return nil
}

func enforceSenderAccess(sender string) error {
	policy, err := LoadAccountPolicy()
	if err != nil {
		return err
	}

	if !policy.hasRules() {
		return nil
	}

	if strings.TrimSpace(sender) == "" {
		return fmt.Errorf("account policy violation: sender identity is required when account policy is active")
	}

	if !policy.matchAllowed(sender) {
		return fmt.Errorf("account policy violation: sender %q is not allowed", sender)
	}

	if policy.matchBlocked(sender) {
		return fmt.Errorf("account policy violation: sender %q is blocked", sender)
	}

	return nil
}

func denyIDOnlyToolWhenPolicyEnabled(toolName string) error {
	policy, err := LoadAccountPolicy()
	if err != nil {
		return err
	}

	if !policy.hasRules() {
		return nil
	}

	return fmt.Errorf("account policy violation: tool %q is disabled while account policy is active because it cannot resolve account identity safely", toolName)
}

func filterListAccountsData(data any) (any, error) {
	policy, err := LoadAccountPolicy()
	if err != nil {
		return nil, err
	}

	if !policy.hasRules() {
		return data, nil
	}

	result, ok := data.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid list_accounts response format")
	}

	accountsAny, ok := result["accounts"].([]any)
	if !ok {
		return nil, fmt.Errorf("invalid list_accounts response: missing accounts array")
	}

	filtered := make([]any, 0, len(accountsAny))
	for _, entry := range accountsAny {
		account, ok := entry.(map[string]any)
		if !ok {
			continue
		}

		name, _ := account["name"].(string)
		emails, _ := account["emailAddresses"].([]any)
		identities := []string{name}
		for _, emailAny := range emails {
			email, _ := emailAny.(string)
			if email != "" {
				identities = append(identities, email)
			}
		}

		blocked := false
		for _, identity := range identities {
			if policy.matchBlocked(identity) {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}

		if len(policy.allowed) == 0 {
			filtered = append(filtered, account)
			continue
		}

		allowed := false
		for _, identity := range identities {
			if policy.matchAllowed(identity) {
				allowed = true
				break
			}
		}
		if allowed {
			filtered = append(filtered, account)
		}
	}

	result["accounts"] = filtered
	result["count"] = len(filtered)
	return result, nil
}

func filterMessagesByAccountField(data any, fieldName string) (any, error) {
	policy, err := LoadAccountPolicy()
	if err != nil {
		return nil, err
	}

	if !policy.hasRules() {
		return data, nil
	}

	result, ok := data.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	messagesAny, ok := result["messages"].([]any)
	if !ok {
		return nil, fmt.Errorf("invalid response: missing messages array")
	}

	filtered := make([]any, 0, len(messagesAny))
	for _, entry := range messagesAny {
		msg, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		account, _ := msg[fieldName].(string)
		if policy.isPermitted(account) {
			filtered = append(filtered, msg)
		}
	}

	result["messages"] = filtered
	result["count"] = len(filtered)
	return result, nil
}
