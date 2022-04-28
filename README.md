[docimg]:https://godoc.org/github.com/ardnew/resvn?status.svg
[docurl]:https://godoc.org/github.com/ardnew/resvn
[repimg]:https://goreportcard.com/badge/github.com/ardnew/resvn
[repurl]:https://goreportcard.com/report/github.com/ardnew/resvn

# resvn
#### Run SVN commands on multiple repositories

[![GoDoc][docimg]][docurl] [![Go Report Card][repimg]][repurl]

```
github.com/ardnew/resvn 0.6.0 linux-amd64 main@cd3d4e6 2022-04-28T15:24:05Z

USAGE
  resvn [flags] [repo-pattern ...] [-- svn-command-line ...]

FLAGS (mnemonics shown in [brackets])
  -c             use [case]-sensitive matching
  -d             print commands which would be executed ([dry-run])
  -f path        use repository definitions from [file] path
  -l user:pass   use user:pass to authenticate with SVN or REST API ([login])
  -o             use logical-[or] matching if multiple patterns given
  -q             suppress all non-essential and error messages ([quiet])
  -s url         prepend [server] url to all constructed URLs
  -u             [update] cached repository definitions from server
  -w             construct [web] URLs instead of repository URLs

NOTES
  All arguments following the first occurrence of "--" are forwarded (in the
  same order they were given) to each "svn" command generated. Since the same
  command may run with multiple repositories, placeholder variables may be used
  in the given command line, which are then substituted with attributes from the
  target repository each time the "svn" command is run.

      @ = repository URL (must be first character in word)
      ^ = repository base name
      & = preceding URL/path argument
      $ = last path component (basename) of "&"
      ! = parent path component (basename of dirname) of "&"

  For example, exporting a common tag from all repositories with "DAPA" in the
  name into respectively-named subdirectories of the current directory:

      % resvn DAPA -- export @/tags/foo ./^/tags/foo
```

### Compile from source code

```sh
# Clone the repository
git clone https://github.com/ardnew/resvn
cd resvn
# Compile executable
go build -v .
# Install somewhere in your $PATH
sudo cp resvn /usr/local/bin
```

### Build and install using only Go toolchain

#### Current Go version 1.16 and later:

```sh
go install -v github.com/ardnew/resvn@latest
```

###### Legacy Go version 1.15 and earlier:

```sh
GO111MODULE=off go get -v github.com/ardnew/resvn
```
