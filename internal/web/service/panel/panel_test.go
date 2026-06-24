package panel

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestPanelHTTPGetUsesConfiguredAuthHeader(t *testing.T) {
	t.Setenv("XUI_DOWNLOAD_AUTH_HEADER", "PRIVATE-TOKEN: test-token")

	var gotAuth string
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotAuth = req.Header.Get("PRIVATE-TOKEN")
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"tag_name":"v9.8.7"}`)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	resp, err := panelHTTPGet(client, "https://gitlab.example/latest")
	if err != nil {
		t.Fatalf("panelHTTPGet: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if gotAuth != "test-token" {
		t.Fatalf("PRIVATE-TOKEN header = %q, want test-token", gotAuth)
	}
}

func TestPanelHTTPGetRejectsInvalidAuthHeader(t *testing.T) {
	t.Setenv("XUI_DOWNLOAD_AUTH_HEADER", "missing-colon")
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatal("request should not be sent with invalid auth header")
		return nil, nil
	})}
	if _, err := panelHTTPGet(client, "https://gitlab.example/latest"); err == nil {
		t.Fatal("expected invalid header error")
	}
}

func TestPanelRawURLUsesConfiguredSource(t *testing.T) {
	t.Setenv("XUI_RAW_BASE_URL", "https://gitlab.example/group/project/-/raw/main/")
	got := panelRawURL("update.sh")
	want := "https://gitlab.example/group/project/-/raw/main/update.sh"
	if got != want {
		t.Fatalf("panelRawURL = %q, want %q", got, want)
	}
}

func TestPanelSourceEnvIncludesConfiguredReleaseSource(t *testing.T) {
	t.Setenv("XUI_RELEASE_API_URL", "https://gitlab.example/api")
	t.Setenv("XUI_RELEASE_TAG_API_URL", "https://gitlab.example/api/tags")
	t.Setenv("XUI_RELEASE_ASSET_URL_TEMPLATE", "https://gitlab.example/{tag}/{arch}.tar.gz")
	t.Setenv("XUI_RAW_BASE_URL", "https://gitlab.example/raw")
	t.Setenv("XUI_XRAY_RELEASE_API_URL", "https://gitlab.example/xray/releases")
	t.Setenv("XUI_XRAY_ASSET_URL_TEMPLATE", "https://gitlab.example/xray/{tag}/Xray-{os}-{arch}.zip")
	t.Setenv("XUI_DOWNLOAD_AUTH_HEADER", "PRIVATE-TOKEN: test-token")

	got := strings.Join(panelSourceEnv(), "\n")
	for _, want := range []string{
		"XUI_RELEASE_API_URL=https://gitlab.example/api",
		"XUI_RELEASE_TAG_API_URL=https://gitlab.example/api/tags",
		"XUI_RELEASE_ASSET_URL_TEMPLATE=https://gitlab.example/{tag}/{arch}.tar.gz",
		"XUI_RAW_BASE_URL=https://gitlab.example/raw",
		"XUI_XRAY_RELEASE_API_URL=https://gitlab.example/xray/releases",
		"XUI_XRAY_ASSET_URL_TEMPLATE=https://gitlab.example/xray/{tag}/Xray-{os}-{arch}.zip",
		"XUI_DOWNLOAD_AUTH_HEADER=PRIVATE-TOKEN: test-token",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("panelSourceEnv missing %q in %q", want, got)
		}
	}
}

func TestIsNewerVersion(t *testing.T) {
	cases := []struct {
		latest  string
		current string
		want    bool
	}{
		{"v2.9.4", "2.9.3", true},
		{"v2.10.0", "2.9.9", true},
		{"v2.9.3", "2.9.3", false},
		{"v2.9.2", "2.9.3", false},
		{"v3.0.0", "2.9.3", true},
	}

	for _, tc := range cases {
		if got := isNewerVersion(tc.latest, tc.current); got != tc.want {
			t.Fatalf("isNewerVersion(%q, %q) = %v, want %v", tc.latest, tc.current, got, tc.want)
		}
	}
}

func TestCompareVersionStringsRejectsUnexpectedFormats(t *testing.T) {
	if _, ok := compareVersionStrings("latest", "2.9.3"); ok {
		t.Fatal("expected non-semver latest tag to be rejected")
	}
	if _, ok := compareVersionStrings("v2.9", "2.9.3"); ok {
		t.Fatal("expected short version to be rejected")
	}
}

func TestShellQuote(t *testing.T) {
	if got := shellQuote("/usr/bin/curl"); got != "'/usr/bin/curl'" {
		t.Fatalf("unexpected quote result: %s", got)
	}
	if got := shellQuote("/tmp/a'b"); got != "'/tmp/a'\\''b'" {
		t.Fatalf("unexpected quote result with single quote: %s", got)
	}
}

func TestExtractReleaseCommit(t *testing.T) {
	full := "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b"
	cases := []struct {
		name    string
		release service.Release
		want    string
	}{
		{
			name:    "from body marker",
			release: service.Release{Body: "Rolling build\n\ncommit=" + full + "\nbuilt=2026-06-24T00:00:00Z"},
			want:    full,
		},
		{
			name:    "body marker is case-insensitive and wins over target",
			release: service.Release{Body: "COMMIT=" + full, TargetCommitish: "deadbeef"},
			want:    full,
		},
		{
			name:    "fallback to target commit sha",
			release: service.Release{Body: "no marker here", TargetCommitish: full},
			want:    full,
		},
		{
			name:    "branch target is not a commit",
			release: service.Release{Body: "no marker", TargetCommitish: "main"},
			want:    "",
		},
	}
	for _, tc := range cases {
		if got := extractReleaseCommit(&tc.release); got != tc.want {
			t.Fatalf("%s: extractReleaseCommit = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestCommitsEqual(t *testing.T) {
	full := "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b"
	cases := []struct {
		a, b string
		want bool
	}{
		{"1a2b3c4d", full, true},  // injected 8-char prefix matches full release sha
		{full, "1a2b3c4d", true},  // order independent
		{"1A2B3C4D", full, true},  // case insensitive
		{"deadbeef", full, false}, // different commit
		{"", full, false},         // empty current never matches
		{"1a2b3c4d", "", false},   // empty latest never matches
	}
	for _, tc := range cases {
		if got := commitsEqual(tc.a, tc.b); got != tc.want {
			t.Fatalf("commitsEqual(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestShortCommit(t *testing.T) {
	if got := shortCommit("1a2b3c4d5e6f7a8b"); got != "1a2b3c4d" {
		t.Fatalf("shortCommit truncation = %q, want %q", got, "1a2b3c4d")
	}
	if got := shortCommit("abc"); got != "abc" {
		t.Fatalf("shortCommit short input = %q, want %q", got, "abc")
	}
}
