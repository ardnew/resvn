package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunUpdateViaSSH(t *testing.T) {
	tempDir := t.TempDir()
	cacheFile := filepath.Join(tempDir, "repos.txt")
	sshScript := writeScript(t, tempDir, "list-repos.sh", "printf ' zebra \\n\\nalpha\\nalpha\\nbeta\\n'")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	err := runMain(
		[]string{"-u", "-f", cacheFile, "-s", "http://svn.example"},
		envLookup(map[string]string{svnSSHIdent: sshScript}),
		stdout,
		stderr,
	)
	if err != nil {
		t.Fatalf("runMain returned error: %v\nstderr=%s", err, stderr.String())
	}

	wantOutput := strings.Join([]string{
		"http://svn.example/svn/alpha",
		"http://svn.example/svn/beta",
		"http://svn.example/svn/zebra",
	}, newline) + newline
	if stdout.String() != wantOutput {
		t.Fatalf("stdout=%q want %q", stdout.String(), wantOutput)
	}

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", cacheFile, err)
	}
	wantCache := "alpha\nbeta\nzebra\n"
	if string(data) != wantCache {
		t.Fatalf("cache contents=%q want %q", string(data), wantCache)
	}
}

func TestRunUpdateMissingSSH(t *testing.T) {
	cacheFile := filepath.Join(t.TempDir(), "repos.txt")
	err := runMain(
		[]string{"-u", "-f", cacheFile, "-s", "http://svn.example"},
		envLookup(nil),
		&bytes.Buffer{},
		&bytes.Buffer{},
	)
	if err == nil || !strings.Contains(err.Error(), "undefined SSH command") {
		t.Fatalf("got err=%v, want undefined SSH command", err)
	}
}

func TestRunUpdateSSHFailure(t *testing.T) {
	tempDir := t.TempDir()
	cacheFile := filepath.Join(tempDir, "repos.txt")
	sshScript := writeScript(t, tempDir, "fail-repos.sh", "echo boom 1>&2\nexit 7")

	err := runMain(
		[]string{"-u", "-f", cacheFile, "-s", "http://svn.example"},
		envLookup(map[string]string{svnSSHIdent: sshScript}),
		&bytes.Buffer{},
		&bytes.Buffer{},
	)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("got err=%v, want stderr from SSH command", err)
	}
}

func TestRunRejectsLegacyRESTEnv(t *testing.T) {
	cacheFile := filepath.Join(t.TempDir(), "repos.txt")
	err := runMain(
		[]string{"-u", "-f", cacheFile, "-s", "http://svn.example"},
		envLookup(map[string]string{legacyAPIIdent: "http://legacy.example"}),
		&bytes.Buffer{},
		&bytes.Buffer{},
	)
	if err == nil {
		t.Fatal("expected error for legacy REST env")
	}
	if !strings.Contains(err.Error(), legacyAPIIdent) || !strings.Contains(err.Error(), svnSSHIdent) {
		t.Fatalf("got err=%v, want migration guidance mentioning %s and %s", err, legacyAPIIdent, svnSSHIdent)
	}
}

func TestRunWebURLsUseConfiguredBase(t *testing.T) {
	tempDir := t.TempDir()
	cacheFile := filepath.Join(tempDir, "repos.txt")
	if err := os.WriteFile(cacheFile, []byte("alpha\nbeta\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", cacheFile, err)
	}

	stdout := &bytes.Buffer{}
	err := runMain(
		[]string{"-f", cacheFile, "-s", "http://svn.example", "-w"},
		envLookup(map[string]string{webURLIdent: "https://browse.example/repos"}),
		stdout,
		&bytes.Buffer{},
	)
	if err != nil {
		t.Fatalf("runMain returned error: %v", err)
	}

	want := strings.Join([]string{
		"https://browse.example/repos/alpha",
		"https://browse.example/repos/beta",
	}, newline) + newline
	if stdout.String() != want {
		t.Fatalf("stdout=%q want %q", stdout.String(), want)
	}
}

func TestRunWebURLsFallBackToLegacyViewVCPath(t *testing.T) {
	tempDir := t.TempDir()
	cacheFile := filepath.Join(tempDir, "repos.txt")
	if err := os.WriteFile(cacheFile, []byte("alpha\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", cacheFile, err)
	}

	stdout := &bytes.Buffer{}
	err := runMain(
		[]string{"-f", cacheFile, "-s", "http://svn.example", "-w"},
		envLookup(nil),
		stdout,
		&bytes.Buffer{},
	)
	if err != nil {
		t.Fatalf("runMain returned error: %v", err)
	}

	want := "http://svn.example/viewvc/alpha" + newline
	if stdout.String() != want {
		t.Fatalf("stdout=%q want %q", stdout.String(), want)
	}
}

func envLookup(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

func writeScript(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	content := "#!/bin/sh\nset -eu\n" + body + "\n"
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
	return path
}
