package config

import "testing"

func TestGetDisplayVersionTaggedBuild(t *testing.T) {
	oldVersion, oldCommit, oldTag := version, commitHash, buildTag
	defer func() {
		version, commitHash, buildTag = oldVersion, oldCommit, oldTag
	}()

	version = "3.3.1"
	commitHash = "abcdef1234567890"
	buildTag = "v3.3.1"

	if got := GetDisplayVersion(); got != "v3.3.1" {
		t.Fatalf("GetDisplayVersion() = %q, want v3.3.1", got)
	}
}

func TestGetDisplayVersionNonTagBuild(t *testing.T) {
	oldVersion, oldCommit, oldTag := version, commitHash, buildTag
	defer func() {
		version, commitHash, buildTag = oldVersion, oldCommit, oldTag
	}()

	version = "3.3.1"
	commitHash = "abcdef1234567890"
	buildTag = ""

	if got := GetDisplayVersion(); got != "abcdef123456" {
		t.Fatalf("GetDisplayVersion() = %q, want abcdef123456", got)
	}
}

func TestGetDisplayVersionFallback(t *testing.T) {
	oldVersion, oldCommit, oldTag := version, commitHash, buildTag
	defer func() {
		version, commitHash, buildTag = oldVersion, oldCommit, oldTag
	}()

	version = "3.3.1"
	commitHash = ""
	buildTag = "v3.3.1"

	if got := GetDisplayVersion(); got != "v3.3.1" {
		t.Fatalf("GetDisplayVersion() = %q, want v3.3.1", got)
	}
}
