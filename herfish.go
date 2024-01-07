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
	LogFormat      string `long:"log-format" choice:"text" choice:"json" default:"text" description:"Log format"`
	Verbose        []bool `short:"v" long:"verbose" description:"Show verbose debug information, each -v bumps log level"`
	logLevel       slog.Level
	Sentinel       string `short:"s" long:"sentinel" default:".git" description:"Sentinel folder to stop searching"`
	CountCommits   bool   `short:"c" long:"count-commits" description:"Count the number of commits in each Git repository"`
	CommitCountMax int    `default:"-1" short:"m" long:"commit-count-max" description:"Filter repositories with commits less than or equal to the specified count"`
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

	if opts.CommitCountMax != -1 && !opts.CountCommits {
		opts.CountCommits = true
	}

	for scanner.Scan() {
		paths = append(paths, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "Error reading input:", err)
		os.Exit(1)
	}

	sort.Strings(paths)

	var dataCollection []templateData
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
			} else if err != nil {
				return fmt.Errorf("failed to count commits: %w", err)
			}

			data.CommitCount = commitCount
		}

		dataCollection = append(dataCollection, data)
	}

	filteredData := applyFilters(dataCollection)

	outputResults(filteredData)

	return nil
}

func applyFilters(dataCollection []templateData) []templateData {
	var filteredData []templateData

	for _, data := range dataCollection {
		if opts.CommitCountMax == -1 {
			filteredData = append(filteredData, data)
		} else if data.CommitCount <= opts.CommitCountMax {
			filteredData = append(filteredData, data)
		}
	}

	return filteredData
}

func outputResults(filteredData []templateData) {
	var resultBuffer bytes.Buffer
	tmpl, err := template.New("output").Parse(outputTemplate)
	if err != nil {
		slog.Error("failed to parse template", "error", err)
		return
	}

	for _, data := range filteredData {
		err := tmpl.Execute(&resultBuffer, data)
		if err != nil {
			slog.Error("failed to execute template", "error", err)
			continue
		}
	}

	fmt.Print(resultBuffer.String())
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

	slog.Debug("counting commits", "repo", repoPath)

	iter, err := repo.Log(&git.LogOptions{})
	if err != nil {
		slog.Debug("failed to query git log", "repo", repoPath)
		return 0, ErrNoGitLog
	}

	count := 0
	err = iter.ForEach(func(commit *object.Commit) error {
		count++
		slog.Debug("found commit", "path", repoPath, "commit", commit.Hash.String())
		return nil
	})
	if err != nil {
		slog.Debug("failed to iterate commits", "path", repoPath)
		return 0, fmt.Errorf("failed to iterate commits: %w", err)
	}

	return count, nil
}
