# resvn

## Run SVN commands on multiple repositories

[![GoDoc][docimg]][docurl] [![Go Report Card][repimg]][repurl]

```text
github.com/ardnew/resvn v0.12.0 darwin-arm64 main@b71c374 2026-05-20T19:05:54Z

A tool for running SVN commands across multiple repositories.

╭──────────────────────────────────────────────────────────────────────────────╮
│  USAGE                                                                       │
╰──────────────────────────────────────────────────────────────────────────────╯

  resvn [flags] [match ...] [! ignore ...] [-- command ...]


   FLAGS • mnemonics shown in [brackets]
  ─────── ───────────────────────────────

  -L string    deprecated: SSH auth is handled by your SSH command
  -S command   use [shell] command to update repository cache via SSH
  -W url       use [web] url to construct browsing URLs
  -a arg       append each [argument] arg to all SVN commands
  -c           use [case]-sensitive matching
  -d           print commands which would be executed ([dry-run])
  -f path      use repository definitions from [file] path
               {"/Users/andrew/.svnrepo"}
  -l string    deprecated: SSH auth is handled by your SSH command
  -o           use logical-[or] matching if multiple patterns given
  -q           suppress all non-essential and error messages ([quiet])
  -s url       use [server] url to construct all URLs {"http://svn.devnet:3690"}
  -u           [update] cached repository definitions from server
  -w           construct [web] URLs instead of repository URLs


   PARAMETERS
  ────────────

  The following parameters are all relative to each URL produced by a given
  search pattern.

   @   repository URL (must prefix a word)
   %   path relative to server root
   ^   repository base name
   &   preceding URL/path argument
   $   last path component (basename) of "&"
   !   parent path component (basename of dirname) of "&"


╭──────────────────────────────────────────────────────────────────────────────╮
│  NOTES                                                                       │
╰──────────────────────────────────────────────────────────────────────────────╯

   SERVICE URLs
  ──────────────

  The default server URL prefix is defined with environment variable $RESVN_URL
  and used when flag "-s" is unspecified.

  The default Web browsing URL prefix is defined with environment variable
  $RESVN_WEB and used when flag "-W" is unspecified. When neither is provided,
  Web URLs default to $RESVN_URL/viewvc.

  The SSH command used to refresh the repository cache is defined with
  environment variable $RESVN_SSH and used when flag "-S" is unspecified. The
  command must print one repository name per line.

  The legacy environment variable $RESVN_API is no longer used for cache
  refresh.

  URLs may include both protocol and port, e.g., "http://server.com:3690".


   PARAMETER EXPANSIONS
  ──────────────────────

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
  ────────────────────

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

## Usage

Run `resvn -h` for the built-in reference. The current output is copied above.

### Update repository cache

`resvn` uses a plain-text cache file with one repository name per line. Each line should contain only the repository base name, not a full URL.

From that cache, `resvn` builds full repository URLs with `-s` or `$RESVN_URL`. If you use `-w`, browse URLs come from `-W` or `$RESVN_WEB`, or fall back to `$RESVN_URL/viewvc`.

Use `-u` to refresh the cache automatically. `resvn` runs the command from `-S` or `$RESVN_SSH`, reads one repository name per line from standard output, normalizes the list, and overwrites the cache file in the current directory. If you keep that file at `${HOME}/.svnrepo`, `resvn` will use it by default when `-f` is not set.

For example:

```sh
RESVN_SSH="ssh -p 22135 -l andrew rstok3-dev02 -- ls -1 /srv/svn/repos" \
  resvn -u -s http://rstok3-dev02
```

`$RESVN_API` is no longer used. For `-u`, `-l` and `-L` are deprecated; handle SSH authentication with your SSH config, agent, or command options instead.

If your browse URL differs from your checkout URL:

```sh
RESVN_URL="http://rstok3-dev02" \
RESVN_WEB="https://rstok3-dev02/svn" \
  resvn -w '^Team'
```

## Install

Choose the install method that fits your workflow. For a given version, they all produce the same tool.

### Download a release

> [Latest release](https://github.com/ardnew/resvn/releases/latest)

Each release includes prebuilt tarballs for common OS and CPU combinations. Use the latest release link above, or browse the full [releases](https://github.com/ardnew/resvn/releases) page for older versions.

### Install with Go

For Go 1.17 and later, use `go install` with an explicit version.

Latest release:

```sh
go install github.com/ardnew/resvn@latest
```

Tip of `main`:

```sh
go install github.com/ardnew/resvn@main
```

Pinned version:

```sh
go install github.com/ardnew/resvn@v0.12.0
```

### Build from source

Clone the repository, build the binary, and install it into a directory on your `PATH`.

```sh
# Clone the repository
git clone https://github.com/ardnew/resvn.git
cd resvn
# Build the executable
go build -o resvn .
# Install it into a directory on your PATH
install -m 0755 resvn /usr/local/bin/resvn
```

[docimg]:https://godoc.org/github.com/ardnew/resvn?status.svg
[docurl]:https://godoc.org/github.com/ardnew/resvn
[repimg]:https://goreportcard.com/badge/github.com/ardnew/resvn
[repurl]:https://goreportcard.com/report/github.com/ardnew/resvn
