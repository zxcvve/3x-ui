package ci_test

import (
	"os"
	"strings"
	"testing"
)

func TestDockerJobUsesDockerCliInsteadOfKanikoOrBuildx(t *testing.T) {
	data, err := os.ReadFile("docker.yml")
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if strings.Contains(content, "gcr.io/kaniko-project/executor") || strings.Contains(content, "/kaniko/executor") {
		t.Fatal("docker:image should use docker buildx instead of Kaniko, which hangs while pushing from the local runner")
	}
	if strings.Contains(content, "docker buildx") || strings.Contains(content, "moby/buildkit") {
		t.Fatal("docker:image should not use buildx because bootstrapping BuildKit pulls an extra Docker Hub image")
	}
	if !strings.Contains(content, "docker build ") || !strings.Contains(content, "docker push ") {
		t.Fatal("docker:image should build and push with the Docker CLI")
	}
}
