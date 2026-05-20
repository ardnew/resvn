// Package cache manages the local repository cache used by resvn.
package cache

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Cache struct {
	FilePath string
	List     []string
}

func findFile(name string, defaultPath string) (path string) {
	exists := func(path string) bool {
		_, err := os.Stat(path)
		return err == nil
	}
	if home, err := os.UserHomeDir(); err == nil {
		if path = filepath.Join(home, name); exists(path) {
			return
		}
	}
	if home, ok := os.LookupEnv("HOME"); ok {
		if path = filepath.Join(home, name); exists(path) {
			return
		}
	}
	if exe, err := os.Executable(); nil == err {
		if path = filepath.Join(filepath.Dir(exe), name); exists(path) {
			return
		}
	}
	if pwd, err := os.Getwd(); nil == err {
		if path = filepath.Join(pwd, name); exists(path) {
			return
		}
	}
	// Could not find file, use the defaultPath if given
	if defaultPath != "" {
		return filepath.Join(defaultPath, name)
	}
	// No default =>> no file
	return ""
}

func New(name string) *Cache {
	return &Cache{
		FilePath: findFile(name, "."),
		List:     []string{},
	}
}

func (c *Cache) Sync(filePath string, update bool, sshCmd string) error {

	c.FilePath = filePath
	c.List = []string{} // clear existing list

	if update {
		sshCmd = strings.TrimSpace(sshCmd)
		if sshCmd == "" {
			return fmt.Errorf("undefined SSH command: set RESVN_SSH or use -S")
		}
		// write all repos to a new temporary file
		tmp, err := c.update(sshCmd)
		if nil != err {
			return err
		}
		// ensure our target repo file path exists
		if err := os.MkdirAll(filepath.Dir(c.FilePath), fs.ModePerm); nil != err {
			return err
		}
		// move the temporary file in place of our given definitions file
		os.Rename(tmp.Name(), c.FilePath)
	}

	file, err := os.Open(c.FilePath)
	if nil != err {
		return err
	}
	defer file.Close()

	scan := bufio.NewScanner(file)

	for scan.Scan() {
		c.List = append(c.List, scan.Text())
	}

	if err := scan.Err(); nil != err {
		return err
	}

	return nil
}

func parseRepoList(r io.Reader) ([]string, error) {
	scan := bufio.NewScanner(r)
	repos := make([]string, 0)
	seen := make(map[string]struct{})
	for scan.Scan() {
		repo := strings.TrimSpace(scan.Text())
		if repo == "" {
			continue
		}
		if strings.ContainsAny(repo, `/\\`) {
			return nil, fmt.Errorf("invalid repository name %q", repo)
		}
		if _, ok := seen[repo]; ok {
			continue
		}
		seen[repo] = struct{}{}
		repos = append(repos, repo)
	}
	if err := scan.Err(); err != nil {
		return nil, err
	}
	sort.Strings(repos)
	return repos, nil
}

func (c *Cache) Match(
	pattern []string, ignore []string, ignoreCase bool) ([]string, error) {

	compile := func(re ...string) ([]*regexp.Regexp, error) {
		x := make([]*regexp.Regexp, len(re))
		for i, p := range re {
			if ignoreCase {
				p = "(?i)" + p
			}
			e, err := regexp.Compile(p)
			if err != nil {
				return nil, err
			}
			x[i] = e
		}
		return x, nil
	}

	expr, err := compile(pattern...)
	if err != nil {
		return nil, err
	}
	cond, err := compile(ignore...)
	if err != nil {
		return nil, err
	}

	m := []string{}
	for _, repo := range c.List {
		// First check if the repo matches ANY ignore pattern
		avoid := false
		for _, e := range cond {
			if avoid = e.MatchString(string(repo)); avoid {
				break // matched an ignore pattern, no need to test others
			}
		}
		if avoid {
			continue // skip this ignored repo
		}
		// Next check if the repo matches ALL select patterns
		match := false
		for _, e := range expr {
			if match = e.MatchString(string(repo)); !match {
				break // did not match some select pattern, no need to test others
			}
		}
		if match {
			// all tests passed, append this repo to returned slice
			m = append(m, string(repo))
		}
	}
	return m, nil
}

func (c *Cache) update(sshCmd string) (*os.File, error) {
	argv := strings.Fields(sshCmd)
	if len(argv) == 0 {
		return nil, fmt.Errorf("undefined SSH command: set RESVN_SSH or use -S")
	}

	cmd := exec.Command(argv[0], argv[1:]...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("%w: %s", err, msg)
		}
		return nil, err
	}

	repos, err := parseRepoList(&stdout)
	if err != nil {
		return nil, err
	}
	log.Printf("received %d repositories\n", len(repos))

	// first write the response to a temporary file and then move it in place of
	// our selected repo definitions file. in case of an error, we won't lose an
	// existing repo definitions file.
	repoFile, err := os.CreateTemp("", filepath.Base(c.FilePath))
	if nil != err {
		return nil, err
	}
	defer repoFile.Close()

	for _, repo := range repos {
		if _, err := fmt.Fprintln(repoFile, repo); nil != err {
			return nil, err
		}
	}

	return repoFile, nil
}
