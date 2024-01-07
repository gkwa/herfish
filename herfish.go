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
	CommitCountMax int    `default:"-1" short:"m" long:"commit-count-max" description:"Filter repositories with commits less than or equal to the specified count"`
}

const outputTemplate = `{{if .CountCommits}}{{printf "%4d %s " .CommitCount .RepoStatus}}{{end}}{{.Dir}}
`

var ErrNoGitLog = errors.New("failed to query git logs")

type templateData struct {
	Dir          string
	CountCommits bool
	CommitCount  int
	RepoStatus   string
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

	slog.Debug("paths", "paths", paths)

	var dataCollection []templateData
	sentinelDirs, err := findSentinelDirs(paths, opts.Sentinel)
	if err != nil {
		return fmt.Errorf("failed to find sentinel dirs: %w", err)
	}

	for _, dir := range sentinelDirs {
		slog.Debug("found sentinel dir", "dir", dir)
	}

	for _, dir := range sentinelDirs {
		data := templateData{
			Dir:          dir,
			CountCommits: opts.CommitCountMax != -1,
			RepoStatus:   "unknown",
		}

		if opts.CommitCountMax != -1 {
			slog.Debug("counting commits", "dir", dir)
			commitCount, err := countCommits(dir)
			if err == ErrNoGitLog {
				slog.Error("no log found", "dir", dir)
			} else if err != nil {
				return fmt.Errorf("failed to count commits: %w", err)
			} else {
				data.CommitCount = commitCount
				slog.Debug("counted commits", "dir", dir, "count", commitCount)
				status, err := getRepoStatus(dir)
				if err != nil {
					return fmt.Errorf("failed to get repo status: %w", err)
				}
				data.RepoStatus = status
			}
		}

		dataCollection = append(dataCollection, data)
	}

	filteredData := applyFilters(dataCollection, opts.CommitCountMax)

	outputResults(filteredData)

	return nil
}

func getRepoStatus(dir string) (string, error) {
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return "", fmt.Errorf("failed to open repo: %w", err)
	}

	isClean, err := isRepoClean(repo)
	if err != nil {
		return "", fmt.Errorf("failed to check repo cleanliness: %w", err)
	}

	if isClean {
		return "clean", nil
	}

	return "dirty", nil
}

func isRepoClean(repo *git.Repository) (bool, error) {
	wt, err := repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("error getting worktree: %w", err)
	}

	status, err := wt.Status()
	if err != nil {
		return false, fmt.Errorf("error getting status: %w", err)
	}

	statusCopy := make(map[string]*git.FileStatus, len(status))
	for k, v := range status {
		statusCopy[k] = v
	}

	for file, s := range status {
		if s.Worktree == git.Untracked {
			delete(statusCopy, file)
		}
	}

	if len(statusCopy) == 0 {
		return true, nil
	}

	return false, nil
}

func applyFilters(dataCollection []templateData, commitCountMax int) []templateData {
	var filteredData []templateData

	for _, data := range dataCollection {
		if commitCountMax == -1 {
			filteredData = append(filteredData, data)
		} else if data.CommitCount <= commitCountMax {
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

func findSentinelDirs(paths []string, sentinelDir string) ([]string, error) {
	uniqueDirs := make(map[string]bool)
	var result []string

	for iter, path := range paths {
		pathInfo, err := os.Stat(path)
		if err != nil {
			return []string{}, fmt.Errorf("failed to stat path: %w", err)
		}

		currentDir, err := filepath.Abs(path)
		if err != nil {
			return []string{}, fmt.Errorf("failed to get absolute path: %w", err)
		}

		if pathInfo.IsDir() && iter != 0 {
			currentDir = filepath.Dir(path)
		}

		slog.Debug("searching for sentinel dir", "path", path, "currentDir", currentDir, "sentinel", sentinelDir)

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

	return result, nil
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
		return nil
	})
	if err != nil {
		slog.Debug("failed to iterate commits", "path", repoPath)
		return 0, fmt.Errorf("failed to iterate commits: %w", err)
	}

	return count, nil
}
