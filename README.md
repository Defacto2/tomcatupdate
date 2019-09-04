# tomcatupdate

### _Defacto2 Apache Tomcat migration tool_

![GitHub](https://img.shields.io/github/license/Defacto2/tomcatupdate?style=flat-square)
![Maintenance](https://img.shields.io/maintenance/no/2017?style=flat-square)
&nbsp;
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
go get github.com/dustin/go-humanize && go get github.com/phayes/permbits
```

Update the const values for both `userID` and `groupID` for the tomcat user and group ids.

- `cat /etc/passwd` will have the user
- `cat /etc/group` will have the group

```bash
cd tomcatupdate
nano tomcatupdate
```

```go
const (
	userID      = 0
	groupID     = 0
)
```

```bash
go build
```

```bash
./tomcatupdate -h
```

```bash
Usage of ./tomcatupdate:
  -dir string
        path to existing Tomcat 8.5 install (default "/opt/tomcat8")
  -log
        log any errors with timestamps
  -quiet
        suppress terminal output
  -ver int
        version of Tomcat 8.5.* to download (default -1)
  -verbose
        detail each file and directory that is handled
```
