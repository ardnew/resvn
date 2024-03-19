package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/ardnew/resvn/cache"
)

var (
	PROJECT   string
	IMPORT    string
	VERSION   string
	BUILDTIME string
	PLATFORM  string
	BRANCH    string
	REVISION  string
)

const (
	cacheName   = ".svnrepo"
	authName    = ".svnauth"
	webURLRoot  = "viewvc"
	svnURLRoot  = "svn"
	svnURLIdent = "RESVN_URL"
	svnAPIIdent = "RESVN_API"
)

var (
	globalArg = []string{"--force-interactive"}
)

const newline = "\r\n"

func exeName() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Base(exe)
	}
	return filepath.Base(os.Args[0])
}

func usage(set *flag.FlagSet) {
	ww := &wordWrap{column: 80, indent: "  ", indentFirst: true}
	fmt.Printf("%s %s %s %s@%s %s"+newline,
		IMPORT, VERSION, PLATFORM, BRANCH, REVISION, BUILDTIME)
	fmt.Println()
	fmt.Println("USAGE")
	fmt.Print(ww.wrap(exeName(), "[flags] [match ...] [! ignore ...] [-- command ...]"))
	fmt.Println()
	fmt.Println("FLAGS (mnemonics shown in [brackets])")
	// Determine the maximum width of the left-hand side containing "-x foo" among
	// all defined flags
	margin := 0
	set.VisitAll(func(f *flag.Flag) {
		name, _ := flag.UnquoteUsage(f)
		if width := len(name) + len(f.Name); width > margin {
			margin = width
		}
	})
	ww.indentFirst = false
	ww.indent = fmt.Sprintf("  %*s ", margin+4, "")
	set.VisitAll(func(f *flag.Flag) {
		name, desc := flag.UnquoteUsage(f)
		flagName := fmt.Sprintf("-%s %s", f.Name, name)
		ww.caption = fmt.Sprintf("  %-*s ", margin+4, flagName)
		if bv, ok := f.Value.(interface{ IsBoolFlag() bool }); !ok || !bv.IsBoolFlag() {
			if f.DefValue != "" {
				desc = strings.Join(
					[]string{desc, fmt.Sprintf("{%q}", f.DefValue)},
					" ",
				)
			}
		}
		fmt.Print(ww.wrap(desc))
	})
	ww.indentFirst = true
	ww.indent = "  "
	ww.caption = ""
	fmt.Println()
	fmt.Println("NOTES")
	fmt.Print(ww.wrap("The default server URL prefix is defined with environment",
		"variable $"+svnURLIdent, "and used when flag \"-s\" is unspecified. ",
		"The default REST API URL prefix is defined with environment variable $"+
			svnAPIIdent, "and used when flag \"-S\" is unspecified. The REST API is",
		"optional because it is only used for automatic generation of the known SVN",
		"repository cache (otherwise given with flag \"-f\").",
		"URLs may include both protocol and port, e.g., \"http://server.com:3690\"."))
	fmt.Println()
	fmt.Print(ww.wrap("All arguments following the first occurrence of \"--\" are",
		"forwarded (in the same order they were given) to each \"svn\" command",
		"generated. Since the same command may run with multiple repositories,",
		"placeholder variables may be used in the given command line, which are then",
		"substituted with attributes from the target repository each time the \"svn\"",
		"command is run."))
	fmt.Println()
	ww.indent = "      "
	fmt.Print(ww.wrap("@ = repository URL (must be first character in word)"))
	fmt.Print(ww.wrap("^ = repository base name"))
	fmt.Print(ww.wrap("& = preceding URL/path argument"))
	fmt.Print(ww.wrap("$ = last path component (basename) of \"&\""))
	fmt.Print(ww.wrap("! = parent path component (basename of dirname) of \"&\""))
	ww.indent = "  "
	fmt.Println()
	fmt.Print(ww.wrap("For example, exporting a common tag from all repositories",
		"with \"DAPA\" in the name (excluding any that match \"Calc\" or \"DIOS\")",
		"into respectively-named subdirectories of the current directory:"))
	fmt.Println()
	ww.indent = "      "
	fmt.Print(ww.wrap("%%", exeName(), "DAPA", "\\!", "Calc", "DIOS", "--",
		"export @/tags/foo ./^/tags/foo"))
	ww.indent = "  "
	fmt.Println()
}

func main() {

	var defBaseURL string
	if url, ok := os.LookupEnv(svnURLIdent); ok {
		defBaseURL = url
	}

	var defRESTAPI string
	if url, ok := os.LookupEnv(svnAPIIdent); ok {
		defRESTAPI = url
	}

	repoCache := cache.New(cacheName, authName)

	//argBrowse := flag.Bool("b", false, "open Web URL with Web browser")
	argCaseSen := flag.Bool("c", false, "use [case]-sensitive matching")
	argDryRun := flag.Bool("d", false, "print commands which would be executed ([dry-run])")
	argRepoFile := flag.String("f", repoCache.FilePath, "use repository definitions from [file] `path`")
	argLogin := flag.String("l", "", "use `user:pass` to authenticate with SVN or REST API ([login])")
	argAuthFile := flag.String("L", repoCache.AuthFilePath, "use file `path` contents as [login] arguments")
	argMatchAny := flag.Bool("o", false, "use logical-[or] matching if multiple patterns given")
	argQuiet := flag.Bool("q", false, "suppress all non-essential and error messages ([quiet])")
	argBaseURL := flag.String("s", defBaseURL, "use [server] `url` to construct all URLs")
	argRESTAPI := flag.String("S", defRESTAPI, "use [server] `url` to construct REST API queries")
	argUpdate := flag.Bool("u", false, "[update] cached repository definitions from server")
	argWebURL := flag.Bool("w", false, "construct [web] URLs instead of repository URLs")
	flag.Usage = func() { usage(flag.CommandLine) }

	flag.Parse()

	if *argQuiet {
		log.SetOutput(io.Discard)
	} else {
		log.SetFlags(log.LstdFlags | log.Lmsgprefix)
		log.SetPrefix("• ")
	}

	// Keep all arguments other than the first occurrence of "--".
	patArg := []string{} // repo-include patterns
	ignArg := []string{} // repo-exclude patterns
	cmdArg := []string{}
	ptrArg := &patArg
	for _, a := range flag.Args() {
		switch {
		case strings.TrimSpace(a) == "--":
			ptrArg = &cmdArg
		case strings.HasPrefix(a, "!"):
			ptrArg = &ignArg
			a = a[1:]
			fallthrough
		default:
			if ptrArg != nil && len(a) > 0 {
				*ptrArg = append(*ptrArg, a)
			}
		}
	}

	parseUserPass := func(str string) (user, pass string, ok bool) {
		s := strings.SplitN(str, ":", 2)
		if ok = len(s) > 1; ok {
			user, pass = s[0], s[1]
		}
		return
	}

	var user, pass string
	tryLogin := func() (ok bool) {
		user, pass, ok = parseUserPass(*argLogin)
		return
	}
	tryAuthFile := func() (ok bool) {
		if f, err := os.Open(*argAuthFile); err == nil {
			defer f.Close()
			s := bufio.NewScanner(f)
			for s.Scan() {
				if user, pass, ok = parseUserPass(s.Text()); ok {
					break
				}
			}
		}
		return
	}

	credsParseError := func(desc, rsrc string) error {
		part := []string{"failed to parse credentials"}
		if desc = strings.TrimSpace(desc); desc != "" {
			part = append(part, desc)
		}
		if rsrc = strings.TrimSpace(rsrc); rsrc != "" {
			part = append(part, fmt.Sprintf("%q", rsrc))
		}
		return fmt.Errorf("%s", strings.Join(part, ": "))
	}

	var errb strings.Builder
	if *argLogin != "" && !tryLogin() {
		errb.WriteString(credsParseError("command-line", *argLogin).Error() + newline)
	}
	if *argAuthFile != "" && !tryAuthFile() {
		errb.WriteString(credsParseError("file", *argAuthFile).Error() + newline)
	}
	if errb.Len() > 0 {
		log.Fatal(errb.String())
	}

	err := repoCache.Sync(*argRepoFile, *argUpdate, *argRESTAPI, user, pass)
	if nil != err {
		log.Fatalln(err)
	}

	if strings.TrimSpace(*argBaseURL) == "" {
		log.Fatalln("undefined server URL: try help (-h)")
	}

	*argBaseURL = strings.TrimRight(*argBaseURL, "/")

	urlRoot := svnURLRoot
	if *argWebURL {
		urlRoot = webURLRoot
	}

	listMatch := func(match []string) {
		for _, repo := range match {
			fmt.Printf("%s/%s/%s", *argBaseURL, urlRoot, repo)
			fmt.Println()
		}
	}

	runMatch := func(match []string) {
		for _, repo := range match {
			url := fmt.Sprintf("%s/%s/%s", *argBaseURL, urlRoot, repo)

			gn := len(globalArg)
			expArg := make([]string, gn+len(cmdArg))
			copy(expArg, globalArg)
			for i, s := range cmdArg {
				prec := ""
				if i > 0 {
					prec = expArg[gn+i-1]
				}
				expArg[gn+i] = expand(s, url, repo, prec)
			}
			// Print the command line being executed
			var cli strings.Builder
			for i, s := range expArg {
				if i > 0 {
					cli.WriteRune(' ')
				}
				if strings.ContainsAny(s, " \t\n$&|<>;`~#{}[]*?!") {
					cli.WriteString("'" + s + "'")
				} else {
					cli.WriteString(s)
				}
			}
			log.Println("» svn " + cli.String())
			if !*argDryRun {
				err := run(expArg...)
				switch {
				case errors.Is(err, &exec.ExitError{}):
					log.Fatalln("error:", string(err.(*exec.ExitError).Stderr))
				case err != nil:
					log.Fatalln("error:", err.Error())
				}
			}
		}
	}

	if len(patArg) == 0 {
		if len(cmdArg) == 0 {
			// no arguments or patterns given, print all known repositories
			for _, repo := range repoCache.List {
				fmt.Printf("%s/%s/%s", *argBaseURL, urlRoot, repo)
				fmt.Println()
			}
		}
	} else {
		if *argMatchAny {
			for _, arg := range patArg {
				match, err := repoCache.Match([]string{arg}, ignArg, !*argCaseSen)
				if nil != err {
					log.Println("warning: skipping invalid expression:", arg)
				}
				if len(cmdArg) == 0 {
					listMatch(match)
				} else {
					runMatch(match)
				}
			}
		} else {
			match, err := repoCache.Match(patArg, ignArg, !*argCaseSen)
			if nil != err {
				log.Fatalln("error: invalid expression(s):",
					"[", strings.Join(patArg, ", "), "]")
			}
			if len(match) == 0 {
				log.Fatalln("error: no repository found matching expression(s):",
					"[", strings.Join(patArg, ", "), "]")
			}
			if len(cmdArg) == 0 {
				listMatch(match)
			} else {
				runMatch(match)
			}
		}
	}
}

func trimTrailingRune(s string, r rune, trim0 bool) string {
	if s == "" {
		return s
	}
	// Iterate over runes instead of bytes for UTF-8 compat.
	su := []rune(s)
	// Find rune count of s without trailing run of r.
	ns := len(su)
	for ; ns > 0; ns-- {
		if su[ns-1] != r {
			break
		}
	}
	switch ns {
	// s consits entirely of r
	case 0:
		if trim0 {
			return "" // Trim sole r at index 0 if trim0
		}
		return string(r) // Keep one r (e.g., root "/")

	// Return s if no trailing run of r was found
	case len(su):
		return s

	// Otherwise, remove trailing run of r
	default:
		return string(su[:ns])
	}
}

func expand(str string, url, base, prec string) string {
	for len(str) > 0 && str[0] == '@' {
		str = url + str[1:]
	}
	prec = trimTrailingRune(prec, '/', false)
	bn := filepath.Base(prec)
	pn := filepath.Base(filepath.Dir(prec))
	return strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(
		strings.ReplaceAll(str, "^", base), "&", prec), "$", bn), "!", pn)
}

func nonEmpty(arg ...string) []string {
	result := make([]string, 0, len(arg))
	for _, s := range arg {
		if t := strings.TrimSpace(s); t != "" {
			result = append(result, s) // keep the untrimmed original
		}
	}
	return result
}

type scribe struct {
	io.Writer
	bytes.Buffer
}

func newScribe(w io.Writer) *scribe { return &scribe{Writer: w} }

func (s *scribe) Write(b []byte) (int, error) {
	s.Buffer.Write(b)
	return s.Writer.Write(b)
}

func run(arg ...string) error {
	stderr := newScribe(os.Stderr)
	cmd := exec.Command("svn", nonEmpty(arg...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = stderr
	cmd.Env = nil // use parent process
	err := cmd.Run()
	if stderr.Len() > 0 {
		if err != nil {
			return fmt.Errorf("%w\r\n%s", err, strings.TrimSpace(stderr.String()))
		}
		return errors.New(strings.TrimSpace(stderr.String()))
	}
	return err
}

type wordWrap struct {
	column      int
	indent      string
	caption     string
	indentFirst bool
}

func unescape(s string) string {
	n := 0
	return strings.Map(func(r rune) rune {
		switch {
		// drop only the first escape rune '\'
		case n == 0 && r == '\\':
			n++
			return rune(-1)
		default:
			return r
		}
	}, s)
}

func (ww *wordWrap) wrap(word ...string) string {
	var sb strings.Builder
	var rp []rune
	for i, w := range word {
		if len(w) > 0 {
			// No visible symbols exist after this word
			last := (i+1 == len(word)) ||
				(strings.TrimSpace(strings.Join(word[i+1:], "")) == "")
			// Word is non-empty
			if t := strings.TrimSpace(w); t != "" {
				// Word contains a visible symbol
				rw, rt := []rune(w), []rune(t)
				// Word is escaped
				escap := len(rw) > 1 && rw[0] == '\\'
				if escap {
					w, t, rw, rt = unescape(w), unescape(t), rw[1:], rt[1:]
				}
				// Word is the first word being added
				first := sb.Len() == 0
				// Word is a punctuation character
				punct := (len(rt) == 1) && unicode.IsPunct(rt[0])
				// Word begins with whitespace
				wsBeg := unicode.IsSpace(rw[0])
				// Previous word ends with whitespace
				wsEnd := (len(rp) > 0) && unicode.IsSpace(rp[len(rp)-1])
				if !first && (!punct || escap) && !wsBeg && !wsEnd {
					sb.WriteRune(' ')
				}
				switch {
				case punct:
					sb.WriteString(t)
				case last:
					sb.WriteString(w[:strings.LastIndex(w, t)+len(t)])
				default:
					sb.WriteString(w)
				}
				rp = rw
			}
			if last {
				break
			}
		}
	}
	var lb strings.Builder
	var ls string
	var ln int
	if ww.indentFirst {
		ls = ww.indent
	} else {
		if ww.caption != "" {
			ls = ww.caption
		} else {
			for _, c := range sb.String() {
				if !unicode.IsSpace(c) {
					break
				}
				ls += string(c)
			}
		}
	}
	lf := strings.Fields(sb.String())
	for i, w := range lf {
		if len(ls)+len(w) >= ww.column {
			lb.WriteString(ls + newline)
			ls = ww.indent
			ln++
			if len(ww.indent)+len(w) >= ww.column {
				ml := ww.column - len(ww.indent) - 1
				for j := 0; j < len(w); j += ml {
					if ml > len(w[j:]) {
						ls += w[j:]
						break
					}
					lb.WriteString(ww.indent + w[j:ml] + "-" + newline)
					ln++
				}
				break
			}
		} else {
			if i > 0 {
				ls += " "
			}
		}
		ls += w
	}
	if ls != ww.indent {
		lb.WriteString(ls + newline)
	}
	return strings.TrimRight(lb.String(), newline) + newline
}
