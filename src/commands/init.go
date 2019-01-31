package commands

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	gitignore "github.com/denormal/go-gitignore"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

const goalignoreFile = ".goalignore"

func getProjectDir() string {
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
	cyan := color.New(color.FgCyan)
	magenta := color.New(color.FgMagenta)

	cyan.Printf("Initializing project\n")
	cyan.Printf("‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾\n")

	magenta.Print("- Template:\t")
	fmt.Println(template)

	magenta.Print("- Destination:\t")
	fmt.Println(dest)

	fmt.Println("")
}

func createIgnoreMatcher(src string) (gitignore.GitIgnore, error) {
	ignoreFilePath := filepath.Join(src, goalignoreFile)
	return gitignore.NewFromFile(ignoreFilePath)
}

func copy(info os.FileInfo, srcPath string, destPath string) error {
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
}

func createProject(src string, dest string) (error, *string) {
	matcher, err := createIgnoreMatcher(src)
	if err != nil {
		return err, nil
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

		return copy(info, srcPath, destPath)
	})

	return err, nil
}

func addTemplateFiles(src string, dest string) (error, *string) {
	templatePath := filepath.Join(src, "_template")

	// Skip if no template folders are present
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		return nil, nil
	}

	return filepath.Walk(templatePath, func(srcPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Build the destination file path
		relativePath, err := filepath.Rel(templatePath, srcPath)
		if err != nil {
			return err
		}
		destPath := filepath.Join(dest, relativePath)

		// Skip the root
		if srcPath == templatePath {
			return nil
		}

		return copy(info, srcPath, destPath)
	}), nil
}

type TaskfileVars struct {
	Pkg     string `yaml:pkg`
	Version string `yaml:version`
}

type Taskfile struct {
	Vars TaskfileVars `yaml:vars`
}

func runCommand(cmd *exec.Cmd) (error, *string) {
	output, err := cmd.CombinedOutput()
	stringOutput := string(output)

	return err, &stringOutput
}

func installTask() (error, *string) {
	cmd := exec.Command("go", "get", "github.com/go-task/task/cmd/task")

	return runCommand(cmd)
}

func initializeModule(dest string, moduleName string) (error, *string) {
	cmd := exec.Command("go", "mod", "init", moduleName)
	cmd.Dir = dest

	return runCommand(cmd)
}

func updateDependencies(dest string) (error, *string) {
	cmd := exec.Command("task", "deps")
	cmd.Dir = dest

	return runCommand(cmd)
}

func install(dest string) (error, *string) {
	cmd := exec.Command("go", "install")
	cmd.Dir = dest

	return runCommand(cmd)
}

func runStep(s *spinner.Spinner, step string, call func() (error, *string)) {
	magenta := color.New(color.FgMagenta)
	bullet := magenta.Sprint("*")
	s.Prefix = fmt.Sprintf("%s %s ", bullet, step)
	s.Color("cyan")

	err, details := call()
	if err != nil {
		red := color.New(color.FgHiRed)
		yellow := color.New(color.FgYellow)

		s.Stop()

		prefix := red.Sprintf("Init step")
		suffix := red.Sprintf("failed\n\n")
		fmt.Printf("%s %s %s", prefix, step, suffix)

		if details != nil {
			yellow.Print(*details)
		} else {
			yellow.Printf("%v\n", err)
		}

		os.Exit(1)
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
			newBinaryName := filepath.Base(pkgPath)
			src := getProjectDir()
			dest := filepath.Base(pkgPath)

			printHeader(src, dest)

			// hide cursor, defer its reappearance
			fmt.Print("\033[?25l")
			defer fmt.Print("\033[?25h")

			s := spinner.New(spinner.CharSets[35], 500*time.Millisecond)
			s.Start()

			runStep(s, "Ensure task is installed", func() (error, *string) {
				return installTask()
			})

			runStep(s, "Create new project", func() (error, *string) {
				return createProject(src, dest)
			})

			runStep(s, "Apply additional template files", func() (error, *string) {
				return addTemplateFiles(src, dest)
			})

			runStep(s, "Initialize module", func() (error, *string) {
				return initializeModule(dest, pkgPath)
			})

			runStep(s, "Update dependencies", func() (error, *string) {
				return updateDependencies(dest)
			})

			runStep(s, "Installing to GOPATH", func() (error, *string) {
				return install(dest)
			})

			s.Stop()
			green := color.New(color.FgGreen, color.Bold)
			cyan := color.New(color.FgCyan)

			prefix := green.Sprintf("Project created successfully! Run")
			command := cyan.Sprintf("cd %s", newBinaryName)
			suffix := green.Sprintf("to start developing")

			fmt.Printf("%s %s %s\n", prefix, command, suffix)
		},
	}

	RegisterCommand(command)
}
