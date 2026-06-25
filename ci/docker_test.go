package ci_test

import (
	"os"
	"strings"
	"testing"
)

func TestKanikoExecutorDoesNotUsePullFlag(t *testing.T) {
	data, err := os.ReadFile("docker.yml")
	if err != nil {
		t.Fatal(err)
	}

	for lineNumber, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "--pull \\" {
			t.Fatalf("line %d uses Kaniko --pull flag, which causes following destinations to be parsed incorrectly", lineNumber+1)
		}
	}
}
