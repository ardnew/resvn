[docimg]:https://godoc.org/github.com/ardnew/resvn?status.svg
[docurl]:https://godoc.org/github.com/ardnew/resvn
[repimg]:https://goreportcard.com/badge/github.com/ardnew/resvn
[repurl]:https://goreportcard.com/report/github.com/ardnew/resvn

# resvn
#### Run SVN commands on multiple repositories

[![GoDoc][docimg]][docurl] [![Go Report Card][repimg]][repurl]

```
github.com/ardnew/resvn 0.11.2 darwin-arm64 main@f42551a 2025-05-15T18:36:37Z

  ╓─────────╖ 
•┊║┊ USAGE ┊║┊
  ╙─────────╜ 

  resvn [flags] [match ...] [! ignore ...] [-- command ...]


         ╭┈┄╌                         ╌┄┈╮
  FLAGS  │ mnemonics shown in [brackets] │
  ─────  ╰┈┄╌                         ╌┄┈╯

  -L path        use file path contents as [login] arguments
                 {"/Users/fsds/.svnauth"}
  -S url         use [server] url to construct REST API queries
                 {"http://svn.devnet:3343"}
  -a arg         append each [argument] arg to all SVN commands
  -c             use [case]-sensitive matching
  -d             print commands which would be executed ([dry-run])
  -f path        use repository definitions from [file] path
                 {"/Users/fsds/.svnrepo"}
  -l user:pass   use user:pass to authenticate with SVN or REST API ([login])
  -o             use logical-[or] matching if multiple patterns given
  -q             suppress all non-essential and error messages ([quiet])
  -s url         use [server] url to construct all URLs
                 {"http://svn.devnet:3690"}
  -u             [update] cached repository definitions from server
  -w             construct [web] URLs instead of repository URLs


  PARAMETERS
  ──────────

   @     repository URL (must be first character in word)
   ^     repository base name
   &     preceding URL/path argument
   $     last path component (basename) of "&"
   !     parent path component (basename of dirname) of "&"



  ╓─────────╖ 
•┊║┊ NOTES ┊║┊
  ╙─────────╜ 

  SERVICE URLs
  ─────── ────

  The default server URL prefix is defined with environment variable $RESVN_URL
  and used when flag "-s" is unspecified.

  The default REST API URL prefix is defined with environment variable
  $RESVN_API and used when flag "-S" is unspecified. The REST API is optional
  because it is only used for automatic generation of the known SVN repository
  cache (otherwise given with flag "-f").

  URLs may include both protocol and port, e.g., "http://server.com:3690".



  PARAMETER EXPANSIONS
  ───────── ──────────

  All arguments following the first occurrence of "--" are forwarded (in the
  same order they were given) to each "svn" command generated.

  Since the same command is used when invoking "svn" for several different
  repository matches (and so the user doesn't have to type fully-qualified
  URLs), placeholder variables may be used in the given command line. These
  variables are then expanded with attributes from each matching repository in
  each relative "svn" command. See PARAMETERS section above.

  For example, exporting a common tag from all repositories with "DAPA" in the
  name (excluding any that match "Calc" or "DIOS") into respectively-named
  subdirectories of the current directory:

      > resvn ^DAPA ! Calc DIOS -- export -r 123 @/tags/foo ./^/tags/foo

  The above can be interpreted as:

      "^DAPA"        ┆ all repositories matching regex /^DAPA/
      "!"            ┆ excluding following patterns:
      "Calc"         ┆   /Calc/
      "DIOS"         ┆   /DIOS/
      "--"           ┆ end of patterns, begin SVN command
      "export"       ┆ run SVN subcommand "export"
      "-r 123"       ┆ revision 123 (export flag "-r")
      "@/tags/foo"   ┆ @ (repo URL) followed by "/tags/foo"
      "./^/tags/foo" ┆ to local dir named "^" (repo base name)

  Assuming the patterns above matched the 3 repositories below, then the above
  command would be expanded to execute the following 3 SVN commands:

      > svn export -r 123 \
            http://server.com:3690/DAPA_Project/tags/foo \
            ./DAPA_Project/tags/foo
      > svn export -r 123 \
            http://server.com:3690/DAPA_Components/tags/foo \
            ./DAPA_Components/tags/foo
      > svn export -r 123 \
            http://server.com:3690/DAPA_Utilities/tags/foo \
            ./DAPA_Utilities/tags/foo



  SVN GLOBAL OPTIONS
  ─── ────── ───────

  Besides the invoked subcommand's options, the "svn" command also recognizes
  several global options that are applicable to all subcommands.

  Shown below, these are provided via environment variable $RESVN_ARG or
  command-line flag "-a". If both are provided, the command-line flag takes
  precedence.

  Multiple global options can be expressed in a single command-line flag's
  argument or by providing the command-line flag multiple times. The following
  examples are all functionally equivalent:

      > resvn -a "--username=foo --password=bar" [...]
      > resvn -a "--username=foo" -a "--password=bar" [...]
      > RESVN_ARG="--username=foo --password=bar" resvn [...]

  The global options are "--force-interactive", by default. If either
  environment variable or command-line flag are provided, they will take
  precedence and omit the default option(s).

```

# Usage

First, please read the command-line usage reference that is output with the help flag `-h` (also copied above).

### Update repository cache

`resvn` operates primarily with a plain-text file, or cache, that defines all known repositories. The format of this file is simply one repository name per line. Each line should be the repository base name *only* (not a full URL). For example, a repository at URL `http://svn.host:3690/svn/Team` would be represented by a single line containing only `Team`. The full URL is generated by `resvn` using a combination of these repository names, the server URL prefix given with flag `-s` (or the default URL from environment variable `$RESVN_URL`), and the presence of flag `-w`.

Instead of creating and maintaining this cache manually, `resvn` is capable of requesting a list of all repositories from the SVN server and updating the cache automatically. When called with flag `-u`, and optionally flag `-l` if SVN credentials are not cached locally, the SVN server REST API is queried and results are written to a cache file in your current directory, overwriting any that already exists. You can copy this repository cache to your ${HOME} directory, and it will be used as the default cache when calling `resvn` without flag `-f`.

# Install

A few different options are available for installing. Version numbers being equal, there's no functional difference between them.

### Download executable

> [Latest release](https://github.com/ardnew/resvn/releases/latest)

A zip package containing the executable and documentation is created for each of the most common OS/arch targets for every released version. Check out the [releases](https://github.com/ardnew/resvn/releases) page to see all versions available. 

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

###### Latest release
```sh
go install -v github.com/ardnew/resvn@latest
```

###### Tip of a branch (active development is `main`)
```sh
go install -v github.com/ardnew/resvn@main
```

###### Unique tag
```sh
go install -v github.com/ardnew/resvn@v0.6.2
```

#### Legacy Go version 1.15 and earlier:

```sh
GO111MODULE=off go get -v github.com/ardnew/resvn
```
