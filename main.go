package main

import (
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
	cacheName      = ".svnrepo"
	webURLRoot     = "viewvc"
	svnURLRoot     = "svn"
	svnURLIdent    = "RESVN_URL"
	webURLIdent    = "RESVN_WEB"
	svnSSHIdent    = "RESVN_SSH"
	legacyAPIIdent = "RESVN_API"
	svnARGIdent    = "RESVN_ARG"
)

type svnArg []string

func (a *svnArg) Set(s string) error {
	if a == nil {
		return errors.New("nil svnArg")
	}
	*a = append(*a, strings.Fields(s)...)
	return nil
}

func (a *svnArg) String() string {
	if a == nil {
		return ""
	}
	return strings.Join(*a, " ")
}

var (
	defaultArg = svnArg{"--force-interactive"}
)

const newline = "\r\n"

func exeName() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Base(exe)
	}
	return filepath.Base(os.Args[0])
}

func usage(out io.Writer, set *flag.FlagSet) {
	ww := &wordWrap{column: 80, indent: "  ", indentFirst: true}
	fmt.Fprintf(out, "%s %s %s %s@%s %s"+newline,
		IMPORT, VERSION, PLATFORM, BRANCH, REVISION, BUILDTIME)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "A tool for running SVN commands across multiple repositories.")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "╭──────────────────────────────────────────────────────────────────────────────╮")
	fmt.Fprintln(out, "│  USAGE                                                                       │")
	fmt.Fprintln(out, "╰──────────────────────────────────────────────────────────────────────────────╯")
	fmt.Fprintln(out)
	fmt.Fprint(out, ww.wrap(exeName(), "[flags] [match ...] [! ignore ...] [-- command ...]"))
	fmt.Fprintln(out)
	fmt.Fprintln(out)
	fmt.Fprintln(out, ww.indent+" FLAGS • mnemonics shown in [brackets]")
	fmt.Fprintln(out, ww.indent+"─────── ───────────────────────────────")
	fmt.Fprintln(out)

	margin := 0
	set.VisitAll(func(f *flag.Flag) {
		name, _ := flag.UnquoteUsage(f)
		if width := len(name) + len(f.Name); width > margin {
			margin = width
		}
	})
	formatDef := func(margin int, name, sym, desc string) string {
		indentFirst, indent, caption := ww.indentFirst, ww.indent, ww.caption
		ww.indentFirst = false
		ww.indent = fmt.Sprintf("  %*s ", margin, "")
		ww.caption = fmt.Sprintf("  %-*s ", margin,
			strings.Join([]string{sym, name}, " "))
		result := ww.wrap(desc)
		ww.indentFirst = indentFirst
		ww.indent = indent
		ww.caption = caption
		return result
	}
	set.VisitAll(func(f *flag.Flag) {
		name, desc := flag.UnquoteUsage(f)
		if bv, ok := f.Value.(interface{ IsBoolFlag() bool }); !ok || !bv.IsBoolFlag() {
			if f.DefValue != "" {
				desc = strings.Join(
					[]string{desc, fmt.Sprintf("{%q}", f.DefValue)},
					" ",
				)
			}
		}
		fmt.Fprint(out, formatDef(margin+4, name, "-"+f.Name, desc))
	})

	fmt.Fprintln(out)
	fmt.Fprintln(out)
	fmt.Fprintln(out, ww.indent+" PARAMETERS")
	fmt.Fprintln(out, ww.indent+"────────────")
	fmt.Fprintln(out)
	fmt.Fprint(out, ww.wrap("The following parameters are all relative to each URL",
		"produced by a given search pattern."))
	fmt.Fprintln(out)
	fmt.Fprint(out, formatDef(margin-4, "@", "", "repository URL (must prefix a word)"))
	fmt.Fprint(out, formatDef(margin-4, "%", "", "path relative to server root"))
	fmt.Fprint(out, formatDef(margin-4, "^", "", "repository base name"))
	fmt.Fprint(out, formatDef(margin-4, "&", "", "preceding URL/path argument"))
	fmt.Fprint(out, formatDef(margin-4, "$", "", "last path component (basename) of \"&\""))
	fmt.Fprint(out, formatDef(margin-4, "!", "", "parent path component (basename of dirname) of \"&\""))
	fmt.Fprintln(out)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "╭──────────────────────────────────────────────────────────────────────────────╮")
	fmt.Fprintln(out, "│  NOTES                                                                       │")
	fmt.Fprintln(out, "╰──────────────────────────────────────────────────────────────────────────────╯")
	fmt.Fprintln(out)
	fmt.Fprintln(out, ww.indent+" SERVICE URLs")
	fmt.Fprintln(out, ww.indent+"──────────────")
	fmt.Fprintln(out)
	fmt.Fprint(out, ww.wrap("The default server URL prefix is defined with environment",
		"variable $"+svnURLIdent, "and used when flag \"-s\" is unspecified."))
	fmt.Fprintln(out)
	fmt.Fprint(out, ww.wrap("The default Web browsing URL prefix is defined with environment",
		"variable $"+webURLIdent, "and used when flag \"-W\" is unspecified.",
		"When neither is provided, Web URLs default to $"+svnURLIdent+"/"+webURLRoot+"."))
	fmt.Fprintln(out)
	fmt.Fprint(out, ww.wrap("The SSH command used to refresh the repository cache is defined",
		"with environment variable $"+svnSSHIdent, "and used when flag \"-S\" is",
		"unspecified. The command must print one repository name per line."))
	fmt.Fprintln(out)
	fmt.Fprint(out, ww.wrap("The legacy environment variable $"+legacyAPIIdent,
		"is no longer used for cache refresh."))
	fmt.Fprintln(out)
	fmt.Fprint(out, ww.wrap("URLs may include both protocol and port, e.g.,",
		"\"http://server.com:3690\"."))
	fmt.Fprintln(out)
	fmt.Fprintln(out)
	fmt.Fprintln(out, ww.indent+" PARAMETER EXPANSIONS")
	fmt.Fprintln(out, ww.indent+"──────────────────────")
	fmt.Fprintln(out)
	fmt.Fprint(out, ww.wrap("All arguments following the first occurrence of \"--\" are",
		"forwarded (in the same order they were given) to each \"svn\" command",
		"generated."))
	fmt.Fprintln(out)
	fmt.Fprint(out, ww.wrap("Since the same command is used when invoking \"svn\" for",
		"several different repository matches (and so the user doesn't have to",
		"type fully-qualified URLs), placeholder variables may be used in the",
		"given command line. These variables are then expanded with attributes",
		"from each matching repository in each relative \"svn\" command. See",
		"PARAMETERS section above."))
	fmt.Fprintln(out)
	fmt.Fprint(out, ww.wrap("For example, exporting a common tag from all repositories",
		"with \"DAPA\" in the name (excluding any that match \"Calc\" or \"DIOS\")",
		"into respectively-named subdirectories of the current directory:"))
	fmt.Fprintln(out)
	ww.indent = "      "
	fmt.Fprint(out, ww.wrap(">", exeName(), "^DAPA", "\\!", "Calc", "DIOS", "--",
		"export -r 123 @/tags/foo ./^/tags/foo"))
	ww.indent = "  "
	fmt.Fprintln(out)
	fmt.Fprint(out, ww.wrap("The above can be interpreted as:"))
	fmt.Fprintln(out)
	ww.indent = "      "
	fmt.Fprintln(out, ww.indent+"\"^DAPA\"        ┆ all repositories matching regex /^DAPA/")
	fmt.Fprintln(out, ww.indent+"\"!\"            ┆ excluding following patterns:")
	fmt.Fprintln(out, ww.indent+"\"Calc\"         ┆   /Calc/")
	fmt.Fprintln(out, ww.indent+"\"DIOS\"         ┆   /DIOS/")
	fmt.Fprintln(out, ww.indent+"\"--\"           ┆ end of patterns, begin SVN command")
	fmt.Fprintln(out, ww.indent+"\"export\"       ┆ run SVN subcommand \"export\"")
	fmt.Fprintln(out, ww.indent+"\"-r 123\"       ┆ revision 123 (export flag \"-r\")")
	fmt.Fprintln(out, ww.indent+"\"@/tags/foo\"   ┆ @ (repo URL) followed by \"/tags/foo\"")
	fmt.Fprintln(out, ww.indent+"\"./^/tags/foo\" ┆ to local dir named \"^\" (repo base name)")
	ww.indent = "  "
	fmt.Fprintln(out)
	fmt.Fprint(out, ww.wrap("Assuming the patterns above matched the 3 repositories",
		"below, then the above command would be expanded to execute the following",
		"3 SVN commands:"))
	fmt.Fprintln(out)
	ww.indent = "      "
	fmt.Fprintln(out, ww.indent+"> svn export -r 123 \\")
	fmt.Fprintln(out, ww.indent+"      http://server.com:3690/DAPA_Project/tags/foo \\")
	fmt.Fprintln(out, ww.indent+"      ./DAPA_Project/tags/foo")
	fmt.Fprintln(out, ww.indent+"> svn export -r 123 \\")
	fmt.Fprintln(out, ww.indent+"      http://server.com:3690/DAPA_Components/tags/foo \\")
	fmt.Fprintln(out, ww.indent+"      ./DAPA_Components/tags/foo")
	fmt.Fprintln(out, ww.indent+"> svn export -r 123 \\")
	fmt.Fprintln(out, ww.indent+"      http://server.com:3690/DAPA_Utilities/tags/foo \\")
	fmt.Fprintln(out, ww.indent+"      ./DAPA_Utilities/tags/foo")
	ww.indent = "  "
	fmt.Fprintln(out)
	fmt.Fprintln(out)
	fmt.Fprintln(out, ww.indent+" SVN GLOBAL OPTIONS")
	fmt.Fprintln(out, ww.indent+"────────────────────")
	fmt.Fprintln(out)
	fmt.Fprint(out, ww.wrap("Besides the invoked subcommand's options, the \"svn\"",
		"command also recognizes several global options that are applicable to all",
		"subcommands."))
	fmt.Fprintln(out)
	fmt.Fprint(out, ww.wrap("Shown below, these are provided via environment variable",
		"$"+svnARGIdent, "or command-line flag \"-a\". If both are provided, the",
		"command-line flag takes precedence."))
	fmt.Fprintln(out)
	fmt.Fprint(out, ww.wrap("Multiple global options can be expressed in a single",
		"command-line flag's argument or by providing the command-line flag",
		"multiple times. The following examples are all functionally equivalent:"))
	fmt.Fprintln(out)
	ww.indent = "      "
	fmt.Fprint(out, ww.wrap(">", exeName(), "-a \"--username=foo --password=bar\"", "[...]"))
	fmt.Fprint(out, ww.wrap(">", exeName(), "-a \"--username=foo\" -a \"--password=bar\"", "[...]"))
	fmt.Fprint(out, ww.wrap(">", svnARGIdent+"=\"--username=foo --password=bar\"", exeName(), "[...]"))
	ww.indent = "  "
	fmt.Fprintln(out)
	fmt.Fprint(out, ww.wrap("The global options are \""+defaultArg.String()+"\", by default.",
		"If either environment variable or command-line flag are provided, they will",
		"take precedence and omit the default option(s)."))
	fmt.Fprintln(out)
}

func main() {
	if err := runMain(os.Args[1:], os.LookupEnv, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runMain(args []string, getenv func(string) (string, bool), stdout io.Writer, stderr io.Writer) error {
	var defBaseURL string
	if url, ok := getenv(svnURLIdent); ok {
		defBaseURL = url
	}

	var defWebBaseURL string
	if url, ok := getenv(webURLIdent); ok {
		defWebBaseURL = url
	}

	var defSSHCmd string
	if cmd, ok := getenv(svnSSHIdent); ok {
		defSSHCmd = cmd
	}

	_, legacyAPISet := getenv(legacyAPIIdent)

	defSVNArgs := defaultArg
	if arg, ok := getenv(svnARGIdent); ok {
		defSVNArgs = svnArg{arg}
	}

	repoCache := cache.New(cacheName)

	var argSVNArgs svnArg
	set := flag.NewFlagSet(exeName(), flag.ContinueOnError)
	set.SetOutput(stderr)
	argCaseSen := set.Bool("c", false, "use [case]-sensitive matching")
	argDryRun := set.Bool("d", false, "print commands which would be executed ([dry-run])")
	argRepoFile := set.String("f", repoCache.FilePath, "use repository definitions from [file] `path`")
	argLogin := set.String("l", "", "deprecated: SSH auth is handled by your SSH command")
	argAuthFile := set.String("L", "", "deprecated: SSH auth is handled by your SSH command")
	argMatchAny := set.Bool("o", false, "use logical-[or] matching if multiple patterns given")
	argQuiet := set.Bool("q", false, "suppress all non-essential and error messages ([quiet])")
	argBaseURL := set.String("s", defBaseURL, "use [server] `url` to construct all URLs")
	argWebBaseURL := set.String("W", defWebBaseURL, "use [web] `url` to construct browsing URLs")
	argSSHCmd := set.String("S", defSSHCmd, "use [shell] `command` to update repository cache via SSH")
	set.Var(&argSVNArgs, "a", "append each [argument] `arg` to all SVN commands")
	argUpdate := set.Bool("u", false, "[update] cached repository definitions from server")
	argWebURL := set.Bool("w", false, "construct [web] URLs instead of repository URLs")
	set.Usage = func() { usage(stderr, set) }

	if err := set.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	if argSVNArgs == nil {
		argSVNArgs = defSVNArgs
	}

	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("• ")
	if *argQuiet {
		log.SetOutput(io.Discard)
	} else {
		log.SetOutput(stderr)
	}

	patArg := []string{}
	ignArg := []string{}
	cmdArg := []string{}
	ptrArg := &patArg
	for _, a := range set.Args() {
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

	if *argUpdate {
		if strings.TrimSpace(*argLogin) != "" || strings.TrimSpace(*argAuthFile) != "" {
			return fmt.Errorf("SSH cache updates use your SSH configuration; -l and -L are no longer supported with -u")
		}
		if strings.TrimSpace(*argSSHCmd) == "" && legacyAPISet {
			return fmt.Errorf("%s is no longer used for cache updates; set %s or use -S", legacyAPIIdent, svnSSHIdent)
		}
	}

	if err := repoCache.Sync(*argRepoFile, *argUpdate, *argSSHCmd); err != nil {
		return err
	}

	if strings.TrimSpace(*argBaseURL) == "" {
		return fmt.Errorf("undefined server URL: try help (-h)")
	}

	*argBaseURL = strings.TrimRight(*argBaseURL, "/")
	if strings.TrimSpace(*argWebBaseURL) != "" {
		*argWebBaseURL = strings.TrimRight(*argWebBaseURL, "/")
	}

	urlPrefix := *argBaseURL + "/" + svnURLRoot
	if *argWebURL {
		switch {
		case strings.TrimSpace(*argWebBaseURL) != "":
			urlPrefix = *argWebBaseURL
		default:
			urlPrefix = *argBaseURL + "/" + webURLRoot
		}
	}

	listMatch := func(match []string) {
		for _, repo := range match {
			fmt.Fprintf(stdout, "%s/%s%s", urlPrefix, repo, newline)
		}
	}

	runMatch := func(match []string) error {
		for _, repo := range match {
			url := fmt.Sprintf("%s/%s", urlPrefix, repo)

			gn := len(argSVNArgs)
			expArg := make([]string, gn+len(cmdArg))
			copy(expArg, argSVNArgs)
			for i, s := range cmdArg {
				prec := ""
				if i > 0 {
					prec = expArg[gn+i-1]
				}
				expArg[gn+i] = expand(s, url, repo, prec)
			}

			var cli strings.Builder
			for i, s := range expArg {
				if i > 0 {
					cli.WriteRune(' ')
				}
				if strings.ContainsAny(s, " \t\n$&|<>;`~#{}[]*?!") {
					cli.WriteString("'")
					cli.WriteString(s)
					cli.WriteString("'")
				} else {
					cli.WriteString(s)
				}
			}
			log.Println("» svn " + cli.String())
			if !*argDryRun {
				if err := runSVN(expArg...); err != nil {
					return fmt.Errorf("error: %w", err)
				}
			}
		}
		return nil
	}

	if len(patArg) == 0 {
		if len(cmdArg) == 0 {
			for _, repo := range repoCache.List {
				fmt.Fprintf(stdout, "%s/%s%s", urlPrefix, repo, newline)
			}
		}
		return nil
	}

	if *argMatchAny {
		for _, arg := range patArg {
			match, err := repoCache.Match([]string{arg}, ignArg, !*argCaseSen)
			if err != nil {
				log.Println("warning: skipping invalid expression:", arg)
				continue
			}
			if len(cmdArg) == 0 {
				listMatch(match)
			} else if err := runMatch(match); err != nil {
				return err
			}
		}
		return nil
	}

	match, err := repoCache.Match(patArg, ignArg, !*argCaseSen)
	if err != nil {
		return fmt.Errorf("error: invalid expression(s): [ %s ]", strings.Join(patArg, ", "))
	}
	if len(match) == 0 {
		return fmt.Errorf("error: no repository found matching expression(s): [ %s ]", strings.Join(patArg, ", "))
	}
	if len(cmdArg) == 0 {
		listMatch(match)
		return nil
	}
	return runMatch(match)
}

func trimTrailingRune(s string, r rune, trim0 bool) string {
	if s == "" {
		return s
	}
	su := []rune(s)
	ns := len(su)
	for ; ns > 0; ns-- {
		if su[ns-1] != r {
			break
		}
	}
	switch ns {
	case 0:
		if trim0 {
			return ""
		}
		return string(r)
	case len(su):
		return s
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

	str = strings.ReplaceAll(str, "^", base)

	if root, ok := strings.CutSuffix(url, base); ok {
		if pr, ok := strings.CutPrefix(prec, root); ok {
			str = strings.ReplaceAll(str, "%", pr)
		}
	}

	str = strings.ReplaceAll(str, "&", prec)
	str = strings.ReplaceAll(str, "$", bn)
	str = strings.ReplaceAll(str, "!", pn)

	return str
}

func nonEmpty(arg ...string) []string {
	result := make([]string, 0, len(arg))
	for _, s := range arg {
		if t := strings.TrimSpace(s); t != "" {
			result = append(result, s)
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

func runSVN(arg ...string) error {
	stderr := newScribe(os.Stderr)
	cmd := exec.Command("svn", nonEmpty(arg...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = stderr
	cmd.Env = nil
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
			last := (i+1 == len(word)) ||
				(strings.TrimSpace(strings.Join(word[i+1:], "")) == "")
			if t := strings.TrimSpace(w); t != "" {
				rw, rt := []rune(w), []rune(t)
				escap := len(rw) > 1 && rw[0] == '\\'
				if escap {
					w, t, rw, rt = unescape(w), unescape(t), rw[1:], rt[1:]
				}
				first := sb.Len() == 0
				punct := (len(rt) == 1) && unicode.IsPunct(rt[0])
				wsBeg := unicode.IsSpace(rw[0])
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
	if ww.indentFirst {
		ls = ww.indent
	} else if ww.caption != "" {
		ls = ww.caption
	} else {
		for _, c := range sb.String() {
			if !unicode.IsSpace(c) {
				break
			}
			ls += string(c)
		}
	}
	lf := strings.Fields(sb.String())
	for i, w := range lf {
		if len(ls)+len(w) >= ww.column {
			lb.WriteString(ls)
			lb.WriteString(newline)
			ls = ww.indent
			if len(ww.indent)+len(w) >= ww.column {
				ml := ww.column - len(ww.indent) - 1
				for j := 0; j < len(w); j += ml {
					if ml > len(w[j:]) {
						ls += w[j:]
						break
					}
					lb.WriteString(ww.indent)
					lb.WriteString(w[j : j+ml])
					lb.WriteString("-")
					lb.WriteString(newline)
				}
				break
			}
		} else if i > 0 {
			ls += " "
		}
		ls += w
	}
	if ls != ww.indent {
		lb.WriteString(ls)
		lb.WriteString(newline)
	}
	return strings.TrimRight(lb.String(), newline) + newline
}
