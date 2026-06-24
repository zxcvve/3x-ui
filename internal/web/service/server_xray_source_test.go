package service

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type serviceRoundTripFunc func(*http.Request) (*http.Response, error)

func (f serviceRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestXrayAssetURLUsesConfiguredTemplate(t *testing.T) {
	t.Setenv("XUI_XRAY_ASSET_URL_TEMPLATE", "https://gitlab.com/zxcvve/xray-core-throttle/-/releases/{tag}/downloads/Xray-{os}-{arch}.zip")
	got := xrayAssetURL("v26.6.1", "linux", "64")
	want := "https://gitlab.com/zxcvve/xray-core-throttle/-/releases/v26.6.1/downloads/Xray-linux-64.zip"
	if got != want {
		t.Fatalf("xrayAssetURL = %q, want %q", got, want)
	}
}

func TestXrayAssetPlatformKeepsUpstreamNames(t *testing.T) {
	tests := []struct {
		goos     string
		goarch   string
		wantOS   string
		wantArch string
	}{
		{"linux", "amd64", "linux", "64"},
		{"linux", "arm64", "linux", "arm64-v8a"},
		{"linux", "armv7", "linux", "arm32-v7a"},
		{"darwin", "amd64", "macos", "64"},
		{"windows", "386", "windows", "32"},
	}
	for _, tt := range tests {
		gotOS, gotArch := xrayAssetPlatform(tt.goos, tt.goarch)
		if gotOS != tt.wantOS || gotArch != tt.wantArch {
			t.Fatalf("xrayAssetPlatform(%q, %q) = (%q, %q), want (%q, %q)",
				tt.goos, tt.goarch, gotOS, gotArch, tt.wantOS, tt.wantArch)
		}
	}
}

func TestFetchXrayVersionsUsesConfiguredClientAndAuth(t *testing.T) {
	t.Setenv("XUI_DOWNLOAD_AUTH_HEADER", "PRIVATE-TOKEN: xray-token")
	t.Setenv("XUI_XRAY_RELEASE_API_URL", "https://gitlab.com/api/v4/projects/1/releases")
	var gotAuth, gotURL string
	client := &http.Client{Transport: serviceRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotAuth = req.Header.Get("PRIVATE-TOKEN")
		gotURL = req.URL.String()
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body: io.NopCloser(strings.NewReader(`[
				{"tag_name":"v26.6.1"},
				{"tag_name":"v26.4.24"},
				{"tag_name":"nightly"}
			]`)),
			Header:  make(http.Header),
			Request: req,
		}, nil
	})}

	got, err := fetchXrayVersions(client, "https://gitlab.com/api/v4/projects/1/releases")
	if err != nil {
		t.Fatalf("fetchXrayVersions: %v", err)
	}
	if strings.Join(got, ",") != "v26.6.1" {
		t.Fatalf("versions = %v, want [v26.6.1]", got)
	}
	if gotAuth != "xray-token" {
		t.Fatalf("PRIVATE-TOKEN header = %q, want xray-token", gotAuth)
	}
	if gotURL != "https://gitlab.com/api/v4/projects/1/releases" {
		t.Fatalf("request URL = %q", gotURL)
	}
}

func TestXrayHTTPGetSkipsAuthHeaderForDefaultSource(t *testing.T) {
	t.Setenv("XUI_DOWNLOAD_AUTH_HEADER", "Authorization: Bearer private-token")
	var gotAuth string
	client := &http.Client{Transport: serviceRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotAuth = req.Header.Get("Authorization")
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(`[]`)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	if _, err := xrayHTTPGet(client, defaultXrayReleaseAPIURL); err != nil {
		t.Fatalf("xrayHTTPGet: %v", err)
	}
	if gotAuth != "" {
		t.Fatalf("Authorization header = %q, want empty for default Xray source", gotAuth)
	}
}

func TestXrayHTTPGetSkipsAuthHeaderForExplicitDefaultTemplate(t *testing.T) {
	t.Setenv("XUI_DOWNLOAD_AUTH_HEADER", "Authorization: Bearer private-token")
	t.Setenv("XUI_XRAY_ASSET_URL_TEMPLATE", defaultXrayAssetURLTemplate)
	var gotAuth string
	client := &http.Client{Transport: serviceRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotAuth = req.Header.Get("Authorization")
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(`[]`)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	if _, err := xrayHTTPGet(client, xrayAssetURL("v26.6.1", "linux", "64")); err != nil {
		t.Fatalf("xrayHTTPGet: %v", err)
	}
	if gotAuth != "" {
		t.Fatalf("Authorization header = %q, want empty for explicit default Xray template", gotAuth)
	}
}

func TestXrayHTTPGetRejectsInvalidAuthHeader(t *testing.T) {
	t.Setenv("XUI_DOWNLOAD_AUTH_HEADER", "missing-colon")
	t.Setenv("XUI_XRAY_RELEASE_API_URL", "https://gitlab.com/api/v4/projects/1/releases")
	client := &http.Client{Transport: serviceRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatal("request should not be sent with invalid auth header")
		return nil, nil
	})}
	if _, err := xrayHTTPGet(client, "https://gitlab.com/api/v4/projects/1/releases"); err == nil {
		t.Fatal("expected invalid header error")
	}
}
