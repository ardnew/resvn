package cache

import (
	"strings"
	"testing"
)

func TestParseRepoList(t *testing.T) {
	repos, err := parseRepoList(strings.NewReader(" zebra \n\nalpha\nalpha\n beta\n"))
	if err != nil {
		t.Fatalf("parseRepoList returned error: %v", err)
	}

	want := []string{"alpha", "beta", "zebra"}
	if len(repos) != len(want) {
		t.Fatalf("got %d repos, want %d (%v)", len(repos), len(want), repos)
	}
	for i := range want {
		if repos[i] != want[i] {
			t.Fatalf("got repos %v, want %v", repos, want)
		}
	}
}

func TestParseRepoListRejectsInvalidNames(t *testing.T) {
	for _, input := range []string{"alpha/beta\n", "alpha\\beta\n"} {
		if _, err := parseRepoList(strings.NewReader(input)); err == nil {
			t.Fatalf("parseRepoList(%q) returned nil error", input)
		}
	}
}
