// Command cache-pkgs caches pacakge directories based on the hash of
// dependency specification file. Unix only.
//
//     Usage:
//        cache-pkgs [opts] <dep-spec-file> <dir> <cmd> [args..]
//
//     Caches output directory (dir) based on the hash of the dependency
//     specification file. If the specification changes the output directory
//     is regenerated using cmd and the args. Useful in CI settings.
//
//     Example:
//        cache-pkgs package.json node_modules npm install
//
//     Options can be:
//       -clean
//         	Clean cache and exit
//       -f	Force remove existing output directory
//       -symlink
//         	Use a symlink instead of copy (default true)
package main

import (
	"crypto/sha1"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strings"
	"time"
)

var (
	symlink = flag.Bool("symlink", true, "Use a symlink instead of copy")
	force   = flag.Bool("f", false, "Force remove existing output directory")
	clean   = flag.Bool("clean", false, "Clean cache and exit")
)

func usage() {
	usageStr := `Usage:
   %s [opts] <dep-spec-file> <dir> <cmd> [args..]

Caches output directory (dir) based on the hash of the dependency
specification file. If the specification changes the output directory
is regenerated using cmd and the args. Useful in CI settings.

Example:
   %s package.json node_modules npm install

Options can be:
`
	me := filepath.Base(os.Args[0])
	fmt.Fprintf(os.Stderr, usageStr, me, me)
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage
	flag.Parse()

	cacheStore, err := cacheDir("")
	if err != nil {
		exitWith("Cache dir problems: ", err)
	}

	if *clean {
		fmt.Printf("Wiping cache %q\n", cacheStore)
		err := os.RemoveAll(cacheStore)
		if err != nil {
			exitWith(err)
		}
		return
	}

	if flag.NArg() < 3 {
		exitUsage("please supply both dependency description file, outputdir and the command to generate it")
	}

	depDesc := flag.Arg(0)
	outputdir := flag.Arg(1)
	cmd := flag.Args()[2]
	args := flag.Args()[3:]

	h, err := hashFile(depDesc)
	if err != nil {
		exitWith("Can't hash dependency description:", err)
	}

	depDir := path.Join(cacheStore, h)

	// pre build
	if *force {
		err := os.RemoveAll(outputdir)
		if err != nil && err != os.ErrNotExist {
			exitWith("Error trying to remove existing output dir", err)
		}
	} else {
		_, err := os.Stat(outputdir)
		if !os.IsNotExist(err) {
			exitWith("output path '", outputdir, "' already exists - maybe rerun with `-f`")
		}
	}

	cached, err := IsDir(depDir)
	if err != nil {
		exitWith("Error looking up cache dir", err)
	}

	// build
	start := time.Now()
	if cached {
		Progress("Found cached dependencies - installing those")
		err = Install(depDir, outputdir, *symlink)
	} else {
		Progressf("Running `%s %s` and caching the output", cmd, strings.Join(args, " "))
		err = GenerateAndCache(depDir, outputdir, cmd, args)
	}

	if err != nil {
		exitWith(err)
	}

	Progressf("Succeeded in %.2f sec", time.Now().Sub(start).Seconds())
}

func Install(from, to string, link bool) (err error) {
	from, err = filepath.Abs(from)
	if err != nil {
		return err
	}
	to, err = filepath.Abs(to)
	if err != nil {
		return err
	}

	if link {
		// to is a symlink to from
		err = os.Symlink(from, to)
	} else {
		err = Copy(from, to)
	}
	return err
}

func IsDir(d string) (bool, error) {
	info, err := os.Stat(d)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return info.IsDir(), nil

}

func run(bin string, args ...string) error {
	cmd := exec.Command(bin, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func GenerateAndCache(cache, outputdir, cmd string, args []string) error {
	err := run(cmd, args...)
	if err != nil {
		return err
	}
	return Copy(outputdir, cache)
}

//
func Copy(a, b string) error {
	return run("cp", "-R", a, b)
}

func exitUsage(a ...interface{}) {
	flag.Usage()
	exitWith(a...)
}
func exitWith(a ...interface{}) {
	fmt.Fprint(os.Stderr, append([]interface{}{"Error: "}, append(a, "\n")...)...)
	os.Exit(1)
}

func hashFile(fname string) (hash string, err error) {
	h := sha1.New()
	f, err := os.Open(fname)
	if err != nil {
		return "", err
	}
	defer f.Close()

	_, err = io.Copy(h, f)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func ensureDir(dir string) error {

	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		Progress("creating cache dir", dir)
		return os.MkdirAll(dir, 0750)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New(dir + " exists but is not a dir")
	}
	return nil
}

func cacheDir(dirName string) (dir string, err error) {

	if dirName == "" {
		dir = dirName
	}

	if dir == "" {
		dir = os.Getenv("CACHE_DIR")
	}

	if dir == "" {
		u, err := user.Current()
		if err != nil {
			return "", err
		}

		dir = path.Join(u.HomeDir, ".dep-cache")
	}

	err = ensureDir(dir)
	if err != nil {
		return "", err
	}
	return dir, nil
}

func Progressf(format string, a ...interface{}) {
	ProgressPrint(fmt.Sprintf(format, a...))
}

func Progress(a ...interface{}) {
	ProgressPrint(fmt.Sprint(a...))
}

func ProgressPrint(s string) {
	prefix := os.Getenv("PREFIX")
	fmt.Fprintf(os.Stderr, "%s%s\n", prefix, s)
}
