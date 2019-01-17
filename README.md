# `tomcatupdate`
_Defacto2 Apache Tomcat migration tool_

[![Go Report Card](https://goreportcard.com/badge/github.com/Defacto2/tomcatupdate)](https://goreportcard.com/report/github.com/Defacto2/tomcatupdate)
[![Build Status](https://travis-ci.org/Defacto2/tomcatupdate.svg?branch=master)](https://travis-ci.org/Defacto2/tomcatupdate)

Usage in Windows has been disabled as it relies on POSIX compatible permission bits. 

[Created in Go](https://golang.org/doc/install), to build from source.

Clone this repo.

```bash
git clone https://github.com/Defacto2/tomcatupdate.git
```

Install the dependencies.

```bash
go get github.com/dustin/go-humanize
go get github.com/phayes/permbits
```

```bash
cd tomcatupdate
go build
```

```bash
./tomcatupdate -h
```
