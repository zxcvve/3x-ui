package ci_test

import (
	"os"
	"strings"
	"testing"
)

func TestDockerJobUsesBuildxInsteadOfKaniko(t *testing.T) {
	data, err := os.ReadFile("docker.yml")
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if strings.Contains(content, "gcr.io/kaniko-project/executor") || strings.Contains(content, "/kaniko/executor") {
		t.Fatal("docker:image should use docker buildx instead of Kaniko, which hangs while pushing from the local runner")
	}
	if !strings.Contains(content, "docker buildx build") {
		t.Fatal("docker:image should build with docker buildx")
	}
}
