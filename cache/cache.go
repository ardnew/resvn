package cache

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-resty/resty/v2"
)

const (
	apiHost     = "rstok3-dev02"
	apiPort     = 3343
	apiProtocol = "http"
	apiFormat   = "json"
	apiVersion  = "1"
	apiURLRoot  = "csvn/api/" + apiVersion
)

const (
	authRealm = "CollabNet Subversion Repository"
)

type Cache struct {
	FilePath     string
	AuthFilePath string
	List         []string
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

func New(name, auth string) *Cache {
	return &Cache{
		FilePath:     findFile(name, "."),
		AuthFilePath: findFile(auth, ""),
		List:         []string{},
	}
}

func (c *Cache) Sync(filePath string, update bool, url string, user string, pass string) error {

	c.FilePath = filePath
	c.List = []string{} // clear existing list

	if update {
		url = strings.TrimSpace(url)
		if url == "" {
			return fmt.Errorf("undefined REST API URL: try help (-h)")
		}
		// write all repos to a new temporary file
		tmp, err := c.update(url, user, pass)
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

type apiRespRepo struct {
	Repo []apiRepo `json:"repositories"`
}

type apiRepo struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	RepoURL string `json:"svnUrl"`
	ViewURL string `json:"viewvcUrl"`
	Status  string `json:"status"`
}

func (c *Cache) update(url string, user string, pass string) (*os.File, error) {

	apiURL := fmt.Sprintf("%s/%s", url, apiURLRoot)

	api := resty.New().
		SetDisableWarn(true).
		SetRetryCount(3).
		SetHostURL(apiURL)

	var err error
	if user == "" && pass == "" {
		user, pass, err = cachedCredentials(authRealm)
	}
	if err == errAgentCachedCredentials {
		log.Printf("using agent-based cached credentials (%s): %s", pass, user)
	} else {
		if err != nil {
			return nil, err
		}
		api.SetBasicAuth(user, pass)
	}

	resp := apiRespRepo{}
	_, err = api.R().
		SetHeader("Accept", "application/"+apiFormat).
		SetQueryParams(map[string]string{"format": apiFormat}).
		SetResult(&resp).
		Get("/repository")
	if nil != err {
		return nil, err
	}
	log.Printf("received %d repositories\n", len(resp.Repo))

	// first write the response to a temporary file and then move it in place of
	// our selected repo definitions file. in case of an error, we won't lose an
	// existing repo definitions file.
	repoFile, err := os.CreateTemp("", filepath.Base(c.FilePath))
	if nil != err {
		return nil, err
	}
	defer repoFile.Close()

	for _, r := range resp.Repo {
		if _, err := fmt.Fprintln(repoFile, r.Name); nil != err {
			return nil, err
		}
	}

	return repoFile, nil
}

var errAgentCachedCredentials = errors.New("credentials cached via authentication agent")

func cachedCredentials(realm string) (user string, pass string, err error) {
	c := exec.Command("svn", "auth", "--show-passwords", realm)
	r, err := regexp.Compile(`(?mi)^(Password|Password cache|Username):\s*(.*)$`)
	if nil != err {
		return "", "", err
	}
	e := errors.New("SVN credentials not cached (see -h for help authenticating)")
	o, err := c.CombinedOutput()
	if nil != err {
		return "", "", e
	}
	s := r.FindAllSubmatch(o, 2)
	if s == nil {
		return "", "", e
	}
	for _, u := range s {
		switch strings.ToLower(string(u[1])) {
		case "username":
			user = string(u[2])
		case "password":
			pass = string(u[2])
		case "password cache":
			pass = string(u[2])
			err = errAgentCachedCredentials
		}
	}
	return
}
