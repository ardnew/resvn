[docimg]:https://godoc.org/github.com/ardnew/resvn?status.svg
[docurl]:https://godoc.org/github.com/ardnew/resvn
[repimg]:https://goreportcard.com/badge/github.com/ardnew/resvn
[repurl]:https://goreportcard.com/report/github.com/ardnew/resvn

# resvn
#### Run SVN commands on multiple repositories

[![GoDoc][docimg]][docurl] [![Go Report Card][repimg]][repurl]

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
