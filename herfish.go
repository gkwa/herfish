package herfish

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"text/template"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/jessevdk/go-flags"
)

var opts struct {
	LogFormat    string `long:"log-format" choice:"text" choice:"json" default:"text" description:"Log format"`
	Verbose      []bool `short:"v" long:"verbose" description:"Show verbose debug information, each -v bumps log level"`
	logLevel     slog.Level
	Sentinel     string `short:"s" long:"sentinel" default:".git" description:"Sentinel folder to stop searching"`
	CountCommits bool   `short:"c" long:"count-commits" description:"Count the number of commits in each Git repository"`
}

const outputTemplate = `{{if .CountCommits}} {{printf "%4d " .CommitCount}}{{end}}{{.Dir}}
`

var ErrNoGitLog = errors.New("failed to query git logs")

type templateData struct {
	Dir          string
	CountCommits bool
	CommitCount  int
}

func Execute() int {
	if err := parseFlags(); err != nil {
		return 1
	}

	if err := setLogLevel(); err != nil {
		return 1
	}

	if err := setupLogger(); err != nil {
		return 1
	}

	if err := run(); err != nil {
		slog.Error("run failed", "error", err)
		return 1
	}

	return 0
}

func parseFlags() error {
	_, err := flags.Parse(&opts)
	return err
}

func run() error {
	fmt.Fprintln(os.Stderr, "Waiting for stdin...")
	scanner := bufio.NewScanner(os.Stdin)
	var paths []string

	for scanner.Scan() {
		paths = append(paths, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "Error reading input:", err)
		os.Exit(1)
	}

	sort.Strings(paths)

	var resultBuffer bytes.Buffer
	sentinelDirs := findSentinelDirs(paths, opts.Sentinel)
	for _, dir := range sentinelDirs {
		data := templateData{
			Dir:          dir,
			CountCommits: opts.CountCommits,
		}

		if opts.CountCommits {
			commitCount, err := countCommits(dir)
			if err == ErrNoGitLog {
				slog.Error("no log found", "dir", dir)
				continue
			}

			data.CommitCount = commitCount
		}

		tmpl, err := template.New("output").Parse(outputTemplate)
		if err != nil {
			return fmt.Errorf("failed to parse template: %w", err)
		}

		err = tmpl.Execute(&resultBuffer, data)
		if err != nil {
			return fmt.Errorf("failed to execute template: %w", err)
		}
	}

	fmt.Print(resultBuffer.String())

	return nil
}

func findSentinelDirs(paths []string, sentinelDir string) []string {
	uniqueDirs := make(map[string]bool)
	var result []string

	for _, path := range paths {
		currentDir := filepath.Dir(path)

		for currentDir != "/" && !uniqueDirs[currentDir] {
			sentinelDir := filepath.Join(currentDir, sentinelDir)
			if _, err := os.Stat(sentinelDir); err == nil {
				result = append(result, currentDir)
				uniqueDirs[currentDir] = true
				break
			}

			currentDir = filepath.Dir(currentDir)
		}
	}

	return result
}

func countCommits(repoPath string) (int, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open repo: %w", err)
	}

	iter, err := repo.Log(&git.LogOptions{})
	if err != nil {
		return 0, ErrNoGitLog
	}

	count := 0
	err = iter.ForEach(func(commit *object.Commit) error {
		count++
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("failed to iterate commits: %w", err)
	}

	return count, nil
}
