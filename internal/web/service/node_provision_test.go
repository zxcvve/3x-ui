package service

import (
	"strings"
	"testing"
)

func TestParseInstallResult(t *testing.T) {
	raw := `
noise
XUI_USERNAME=admin
XUI_PASSWORD=secret
XUI_PANEL_PORT=54321
XUI_WEB_BASE_PATH=panel
XUI_ACCESS_URL=http://203.0.113.10:54321/panel
XUI_API_TOKEN=tok_123
`
	got, err := parseInstallResult(raw)
	if err != nil {
		t.Fatalf("parseInstallResult: %v", err)
	}
	if got.PanelPort != 54321 {
		t.Fatalf("PanelPort = %d, want 54321", got.PanelPort)
	}
	if got.WebBasePath != "panel" {
		t.Fatalf("WebBasePath = %q, want panel", got.WebBasePath)
	}
	if got.AccessURL != "http://203.0.113.10:54321/panel" {
		t.Fatalf("AccessURL = %q", got.AccessURL)
	}
	if got.APIToken != "tok_123" {
		t.Fatalf("APIToken = %q", got.APIToken)
	}
}

func TestParseInstallResultRequiresToken(t *testing.T) {
	if _, err := parseInstallResult("XUI_PANEL_PORT=54321\n"); err == nil {
		t.Fatal("expected missing token error")
	}
}

func TestRedactProvisionOutput(t *testing.T) {
	lines := redactProvisionOutput(strings.Join([]string{
		"XUI_USERNAME=admin",
		"XUI_PASSWORD=secret",
		"XUI_API_TOKEN=token",
		"SSH_PRIVATE_KEY=key",
	}, "\n"))
	joined := strings.Join(lines, "\n")
	if strings.Contains(joined, "secret") || strings.Contains(joined, "token") || strings.Contains(joined, "key") {
		t.Fatalf("output was not redacted: %s", joined)
	}
	if !strings.Contains(joined, "XUI_USERNAME=admin") {
		t.Fatalf("non-secret line missing: %s", joined)
	}
}

func TestNormalizeSSHFingerprint(t *testing.T) {
	fp := "gIlayuW+JE9lHgJR3jhu2V036MqWLcRfsID8ZB8+HWM"
	want := fp + "="
	tests := []string{
		" SHA256:" + fp + " ",
		"256 SHA256:" + fp + " no comment (ED25519)",
		"U0hBMjU2OmdJbGF5dVcrSkU5bEhnSlIzamh1MlYwMzZNcVdMY1Jmc0lEOFpCOCtIV00K",
	}
	for _, input := range tests {
		got := normalizeSSHFingerprint(input)
		if got != want {
			t.Fatalf("normalizeSSHFingerprint(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestProvisionRequestNormalizeRequiresHostKey(t *testing.T) {
	req := &NodeProvisionRequest{
		Name:        "n1",
		SSHHost:     "203.0.113.10",
		SSHPort:     22,
		SSHUser:     "root",
		SSHPassword: "pw",
	}
	if err := req.normalize(); err == nil {
		t.Fatal("expected missing host key fingerprint error")
	}
}

func TestProvisionRequestNormalizeAllowsSkippingHostKeyCheck(t *testing.T) {
	req := &NodeProvisionRequest{
		Name:                "n1",
		SSHHost:             "203.0.113.10",
		SSHPort:             22,
		SSHUser:             "root",
		SSHPassword:         "pw",
		SSHHostKeySHA256:    "stale value",
		SSHSkipHostKeyCheck: true,
	}
	if err := req.normalize(); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if req.SSHHostKeySHA256 != "" {
		t.Fatalf("SSHHostKeySHA256 = %q, want empty when check is skipped", req.SSHHostKeySHA256)
	}
}
