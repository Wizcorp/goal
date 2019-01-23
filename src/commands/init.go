package commands

import (
	"fmt"
	"go/build"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	gitignore "github.com/denormal/go-gitignore"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

const goalignoreFile = ".goalignore"

func getTemplateDir() string {
	var returnPath string

	gopath := runtime.GOROOT()

	for i := 0; ; i++ {
		_, file, _, ok := runtime.Caller(i)
		path := filepath.Dir(file)

		if !ok || strings.HasPrefix(path, gopath) {
			return returnPath
		}

		returnPath = path
	}
}

func printHeader(template string, dest string) {
	binaryName := os.Args[0]

	cyan := color.New(color.FgCyan)
	magenta := color.New(color.FgMagenta)

	cyan.Printf("Creating project %s\n\n", binaryName)

	magenta.Print("template:\t")
	fmt.Println(template)

	magenta.Print("destination:\t")
	fmt.Println(dest)

	fmt.Println("")
}

func getGoPath() string {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = build.Default.GOPATH
	}

	return gopath
}

func createIgnoreMatcher(src string) (gitignore.GitIgnore, error) {
	ignoreFilePath := filepath.Join(src, goalignoreFile)
	return gitignore.NewFromFile(ignoreFilePath)
}

func createFromTemplate(src string, dest string) error {
	matcher, err := createIgnoreMatcher(src)
	if err != nil {
		return err
	}

	err = filepath.Walk(src, func(srcPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Build the destination file path
		relativePath, err := filepath.Rel(src, srcPath)
		if err != nil {
			return err
		}
		destPath := filepath.Join(dest, relativePath)

		// Skip the root
		if srcPath == src {
			return os.Mkdir(destPath, info.Mode())
		}

		// Skip file if parent has been ignore
		destBasename := filepath.Dir(destPath)
		if _, err := os.Stat(destBasename); os.IsNotExist(err) {
			return nil
		}

		// Ignore file based on .goalignore file
		match := matcher.Relative(relativePath, info.IsDir())
		if match != nil {
			if match.Ignore() {
				return nil
			}
		}

		// Create a directory if the source is a directory
		if info.IsDir() {
			return os.Mkdir(destPath, info.Mode())
		}

		// Copy file
		srcFile, err := os.Open(srcPath)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		destFile, err := os.OpenFile(destPath, os.O_RDWR|os.O_CREATE, info.Mode())
		if err != nil {
			return err
		}
		defer destFile.Close()

		_, err = io.Copy(destFile, srcFile)
		return err
	})

	return err
}

type TaskfileVars struct {
	Pkg     string `yaml:pkg`
	Version string `yaml:version`
}

type Taskfile struct {
	Vars TaskfileVars `yaml:vars`
}

func updateTaskFile(dest string, pkg string) error {
	taskfilePath := filepath.Join(dest, "Taskfile.yml")
	data, err := ioutil.ReadFile(taskfilePath)
	if err != nil {
		return err
	}

	var reStr *regexp.Regexp
	output := string(data)

	reStr = regexp.MustCompile("(?m:^  pkg:.*$)")
	output = reStr.ReplaceAllString(output, "  pkg: "+pkg)

	reStr = regexp.MustCompile("^  version:.*/m")
	output = reStr.ReplaceAllString(output, "  version: 0.0.1")

	return ioutil.WriteFile(taskfilePath, []byte(output), 0644)
}

func updateVendorDependencies(dest string) error {
	cmd := exec.Command("dep", "ensure")
	cmd.Dir = dest

	return cmd.Run()
}

func runStep(s *spinner.Spinner, prefix string, call func() error) {
	cyan := color.New(color.FgMagenta)
	bullet := cyan.Sprint("*")
	s.Prefix = fmt.Sprintf("%s %s ", bullet, prefix)
	s.Color("cyan")

	err := call()
	if err != nil {
		s.Stop()
		panic(err)
	}
}

func init() {
	command := &Command{
		Use:   "init [destination folder]",
		Args:  cobra.MinimumNArgs(1),
		Short: "Initialize a new project",
		Long:  `Initialize a new project`,
		Run: func(cmd *cobra.Command, args []string) {
			pkgPath := args[0]
			src := getTemplateDir()
			dest := filepath.Join(getGoPath(), "src", pkgPath)

			printHeader(src, dest)

			// hide cursor, defer its reappearance
			fmt.Print("\033[?25l")
			defer fmt.Print("\033[?25h")

			s := spinner.New(spinner.CharSets[35], 100*time.Millisecond)
			s.Start()

			runStep(s, "Create new project from template", func() error {
				return createFromTemplate(src, dest)
			})

			runStep(s, "Update Taskfile.yml", func() error {
				return updateTaskFile(dest, pkgPath)
			})

			runStep(s, "Update vendor dependencies", func() error {
				return updateVendorDependencies(dest)
			})

			s.Stop()
			green := color.New(color.FgGreen, color.Bold)
			green.Printf("Project created successfully\n")
		},
	}

	AddCommand(command)
}
