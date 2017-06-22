// tomcatupdate.go - Defacto2 Apache Tomcat migration tool
// version 1.0
// © Ben Garrett
//
// References:
// Golang: Working with Gzip and Tar http://blog.ralch.com/tutorial/golang-working-with-tar-and-gzip/
// https://golang.org/pkg/net/http/#Response

package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto"
	"crypto/sha1"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/phayes/permbits"
)

const (
	ver1    = "8" // Tomcat major version
	ver2    = "5" // Tomcat minor version
	userID  = 106 // `tomcat7` user ID (cat /etc/passwd)
	groupID = 114 // `tomcat7` group ID (cat /etc/group)
	prefix  = "." // Text to separate results from other feedback

	urlTemplate = "http://www.apache.org/dist/tomcat/tomcat-?/v?/bin/?apache-tomcat-?.tar.gz" // Must always point to apache.org and not a host mirror
)

var (
	conf      = "conf"         // Tomcat configuration sub-directory
	logErrs   = false          // Log errors with a timestamp
	quiet     = false          // No terminal output except for errors
	tomcatDir = "/opt/tomcat8" // Location of Tomcat installation
	verbose   = false          // Output each archive item handled
	ver3      = -1             // Tomcat point version

	configs = []string{"logging.properties", "server.xml", "web.xml"}                                                                      // Tomcat configurations to migrate
	ignored = []string{"LICENSE", "NOTICE", "webapps/docs", "webapps/examples", "webapps/host-manager", "webapps/manager", "webapps/ROOT"} // Ignore these directories and files when extracting from tarball
	urlPage = fmt.Sprintf("http://tomcat.apache.org/download-%v0.cgi", ver1)                                                               // Link to Apache Tomcat download page
)

func init() {
	if runtime.GOOS == "windows" {
		err := fmt.Errorf("This application is not compatible with Microsoft Windows")
		checkErr(err)
	}
}

func main() {
	// handle command line options
	logErrsFlag := flag.Bool("log", logErrs, fmt.Sprintf("log any errors with timestamps"))
	tomcatDirFlag := flag.String("dir", tomcatDir, fmt.Sprintf("path to existing Tomcat %v.%v install", ver1, ver2))
	quietFlag := flag.Bool("quiet", quiet, fmt.Sprintf("suppress terminal output"))
	verFlag := flag.Int("ver", -1, fmt.Sprintf("version of Tomcat %v.%v.* to download", ver1, ver2))
	verboseFlag := flag.Bool("verbose", false, fmt.Sprintf("detail each file and directory that is handled"))
	flag.Parse()
	logErrs = *logErrsFlag
	quiet = *quietFlag
	tomcatDir = *tomcatDirFlag
	verbose = *verboseFlag
	verF := *verFlag

	// check for existence of the Tomcat path
	_, err := os.Stat(tomcatDir)
	if os.IsNotExist(err) {
		if quiet != true {
			err = fmt.Errorf("The path to Tomcat %q cannot be found, please supply a different directory using --dir (directory)", tomcatDir)
		}
		checkErr(err)
	}

	// ask for Tomcat version if no valid flag is supplied
	if verF == -1 {
		fmt.Printf("Which version of Tomcat %v.%v.* do you wish to download?: ", ver1, ver2)
		ver3, err = askVer()
		// loop to keep asking for valid input
		for err != nil {
			ver3, err = askVer()
		}
	} else {
		ver3 = verF
	}

	// build URL to download Tomcat
	f := strings.Split(urlTemplate, "?")
	dirname := fmt.Sprintf("%v%v.%v.%v", f[3], ver1, ver2, ver3)
	filename := fmt.Sprintf("%v%v.%v.%v%v", f[3], ver1, ver2, ver3, f[4])
	srcFile := fmt.Sprintf("%v%v%v%v.%v.%v%v%v", f[0], ver1, f[1], ver1, ver2, ver3, f[2], filename)
	srcSha1 := fmt.Sprintf("%v.sha1", srcFile)
	if quiet == false {
		fmt.Printf("Will download Tomcat %v.%v.%v from URL: %v", ver1, ver2, ver3, srcFile)
	}

	// checksums
	var lcs string              // local file checksum
	rcs := getChecksum(srcSha1) // remote checksum hosted on tomcat.apache.org

	// handle any local files with the same Tomcat archive filename
	lfn, err := os.Open(filename)
	defer lfn.Close()
	if err == nil {
		// if local file exists, check its SHA1 checksum against the one
		// hosted on tomcat.apache.org
		lfh := crypto.SHA1.New()
		io.Copy(lfh, lfn)
		lcs = strings.Split(fmt.Sprintf("%x", lfh.Sum(nil)), "*")[0]
	}

	// download remote Tomcat archive unless an identical local file already exists
	if lcs != rcs {
		download(filename, srcFile, rcs)
	} else if quiet == false {
		fmt.Printf("%v skipped file exists", prefix)
	}

	// unpack tar.gz archive
	tar := openGZip(filename, "")
	// unpack tarball
	_ = openTAR(tar, "")

	// migrate existing configurations
	cp(dirname, conf, configs...)

	if runtime.GOOS != "windows" {
		f := filepath.Join(dirname, conf)
		// chmod g+wrx conf
		mod, err := permbits.Stat(f)
		checkErr(err)
		if !mod.GroupWrite() {
			mod.SetGroupWrite(true)
			err := permbits.Chmod(f, mod)
			checkErr(err)
		}
		if !mod.GroupRead() {
			mod.SetGroupRead(true)
			err := permbits.Chmod(f, mod)
			checkErr(err)
		}
		if !mod.GroupExecute() {
			mod.SetGroupExecute(true)
			err := permbits.Chmod(f, mod)
			checkErr(err)
		}
		// chown -R tomcat7:tomcat7
		if quiet == false {
			fmt.Printf("\nChange ownership of %v/ to user ID %v and group ID %v", dirname, userID, groupID)
		}
		err = changeOwner(dirname, true, userID, groupID)
		checkErr(err)
		if verbose == false && quiet == false {
			fmt.Printf("%v done", prefix)
		}
		// create symbolic links
		t := "/var/www/defacto2.2014/WEB-INF/web.xml"
		sym := filepath.Join(dirname, "conf/lucee.xml")
		createLink(t, sym)
		t = "/var/www/defacto2.2014"
		sym = filepath.Join(dirname, "webapps/ROOT/")
		createLink(t, sym)
		// create tomcat8 symbolic link
		if _, err := os.Stat("tomcat8"); err == nil {
			err = os.Rename("tomcat8", "tomcat8~")
		}
		createLink(dirname, "tomcat8")
	}
	if quiet == false {
		fmt.Printf("\nTomcat update complete\n")
	}
}

func askVer() (int, error) {
	reader := bufio.NewReader(os.Stdin)
	i, _ := reader.ReadString('\n')
	i = strings.Trim(i, "\n\r")
	ver3, err := strconv.Atoi(i)
	if err != nil {
		fmt.Printf("\rThe version number needs to be a digit: ")
	}
	return ver3, err
}

func changeOwner(dir string, recursive bool, uID, gID int) error {
	if recursive == false {
		err := os.Chown(dir, uID, gID)
		return err
	}
	var c int
	return filepath.Walk(dir, func(name string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		c++
		err = os.Chown(name, uID, gID)
		if verbose == true {
			fmt.Printf("\n%v. %v", c, name)
			if err != nil {
				fmt.Printf("%v failed", prefix)
			}
		}
		return nil
	})
}

func calcSHA1(filePath string) ([]byte, error) {
	var result []byte
	file, err := os.Open(filePath)
	if err != nil {
		return result, err
	}
	defer file.Close()

	hash := sha1.New()
	if _, err := io.Copy(hash, file); err != nil {
		return result, err
	}

	return hash.Sum(result), nil
}

func cp(rootDir string, subDir string, files ...string) {
	inFile, outFile := "", ""
	inDir := filepath.Join(rootDir, subDir)
	outDir := filepath.Join(tomcatDir, subDir)

	for _, f := range files {
		inFile = filepath.Join(outDir, f)
		outFile = filepath.Join(inDir, f)
		if quiet == false {
			fmt.Printf("\n%v will be replaced", outFile)
		}

		inCS, err := calcSHA1(inFile)
		checkErr(err)

		info, err := os.Stat(inFile)
		checkErr(err)

		if !info.Mode().IsRegular() {
			err = fmt.Errorf("%v is not a valid file", inFile)
		}
		checkErr(err)

		in, err := os.Open(inFile)
		checkErr(err)
		defer in.Close()

		out, err := os.Create(outFile)
		checkErr(err)
		defer out.Close()

		_, err = io.Copy(out, in) // _ returns file size
		checkErr(err)

		err = out.Sync()
		checkErr(err)

		outCS, err := calcSHA1(outFile)
		checkErr(err)

		if fmt.Sprint(outCS) != fmt.Sprint(inCS) {
			err = fmt.Errorf("%v did not copy correctly, aborting", inFile)
		}
		checkErr(err)

		if quiet == false {
			fmt.Printf("%v done", prefix)
		}
	}
}

func createLink(target, symlink string) {
	err := os.Symlink(target, symlink)
	if quiet == false {
		fmt.Printf("\nSymlink %v → %v", symlink, target)
		if err != nil {
			// display instead of log errors
			es := strings.Fields(fmt.Sprint(err))
			fmt.Printf("%v skipped %v", prefix, strings.Join(es[3:], " ")) // fetch and append error reason
		}
	}
}

func download(filename string, url string, checksum string) {
	// create a local file to save download to
	lfn, err := os.Create(filename)
	checkErr(err)
	defer lfn.Close()
	// download remote file metadata
	head, err := http.Head(url)
	checkErr(err)
	checkHTTP(head)
	if quiet == false {
		fmt.Printf("\nDownloading file: %v, %v", filename, humanize.Bytes(uint64(head.ContentLength)))
		lm := head.Header.Get("Last-Modified")
		if len(lm) != 0 {
			fmt.Printf(", %v\n", lm)
		}
	}
	// download remote file data
	resp, err := http.Get(url)
	checkErr(err)
	// save download to local file
	defer resp.Body.Close()
	_, err = io.Copy(lfn, resp.Body)
	checkErr(err)
	// validate the download after it is complete
	calc, err := calcSHA1(filename)
	checkErr(err)
	ccs := fmt.Sprintf("%x", calc)
	if ccs != checksum {
		err := fmt.Errorf("The download failed as the checksum of %v does not match the expected checksum\nExpected: %q\n  Actual: %q", filename, checksum, ccs)
		checkErr(err)
	} else {
		if quiet == false {
			fmt.Println("Download complete")
		}
	}
}

func openTAR(source, target string) string {
	// open tarball
	if quiet == false {
		fmt.Printf("\nTarball content extraction")
	}
	reader, err := os.Open(source)
	checkErr(err)
	defer func() {
		reader.Close()
		if verbose == true {
			fmt.Printf("\nCompleted tarball content extraction")
		} else if quiet == false {
			fmt.Printf("%v done", prefix)
		}
	}()
	// loop and read through tarball
	c, tar, dir := 0, tar.NewReader(reader), ""
	var skip bool
	var spl []string
	var chk string
	for {
		head, err := tar.Next()
		if err == io.EOF {
			break
		} else {
			checkErr(err)
		}
		// get item (dir or file)
		dir = filepath.Join(target, head.Name)
		info := head.FileInfo()
		c++
		if verbose == true {
			fmt.Printf("\n%v. %v", c, head.Name)
		}
		// skip items that are to be ignored
		spl = strings.Split(head.Name, "/")
		if len(spl) >= 3 {
			chk = fmt.Sprintf("%v/%v", spl[1], spl[2])
		} else if len(spl) == 2 {
			chk = fmt.Sprintf("%v", spl[1])
		}
		skip = false
		for _, p := range ignored {
			if chk == p {
				skip = true
				continue
			}
		}
		if skip == true {
			if verbose == true {
				fmt.Printf("%v skipped", prefix)
			}
			continue
		}
		// handle (create) directories
		if info.IsDir() {
			if err = os.MkdirAll(dir, info.Mode()); err != nil {
				checkErr(err)
			}
			continue
		}
		// handle (copy) files
		file, err := os.OpenFile(dir, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
		checkErr(err)
		defer file.Close()
		_, err = io.Copy(file, tar)
		checkErr(err)
	}
	return strings.TrimSuffix(source, filepath.Ext(source))
}

func openGZip(source, target string) string {
	// open gzip archive (only supports a single file extraction)
	reader, err := os.Open(source)
	checkErr(err)
	defer reader.Close()
	// read archive
	gz, err := gzip.NewReader(reader)
	checkErr(err)
	defer gz.Close()
	// create a filename
	if len(gz.Name) == 0 {
		ext := path.Ext(source)
		gz.Name = source[0 : len(source)-len(ext)]
	}
	if len(target) != 0 {
		target = filepath.Join(target, gz.Name)
	} else {
		target = gz.Name
	}
	if quiet == false {
		fmt.Printf("\nTarball %v", target)
	}
	// create an empty file
	writer, err := os.Create(target)
	checkErr(err)
	defer func() {
		writer.Close()
		if quiet == false {
			fmt.Printf("%v created", prefix)
		}
	}()
	// save extracted tarball to empty file
	_, err = io.Copy(writer, gz)
	checkErr(err)
	return target
}

func getChecksum(url string) string {
	resp, err := http.Get(url)
	checkErr(err)
	checkHTTP(resp)
	// Save download to local file
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	checkErr(err)
	cs := string(strings.Split(fmt.Sprintf("%s", data), "*")[0])
	cs = strings.TrimSpace(cs)
	return cs
}

func checkErr(err error) {
	if err != nil {
		if logErrs == true {
			log.Fatal("ERROR: ", err)
		} else {
			fmt.Printf("\n%s\n", err)
			os.Exit(0)
		}
	}
}

func checkHTTP(r *http.Response) {
	if r.StatusCode != 200 {
		err := fmt.Errorf("%v. Maybe check %v for the current version?", r.Status, urlPage)
		if logErrs == true {
			log.Fatal("SERVER ERROR: ", err)
		} else {
			fmt.Printf("\n%s\n", err)
			os.Exit(0)
		}
	}
}
