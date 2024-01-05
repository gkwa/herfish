package herfish

import (
	"bufio"
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/jessevdk/go-flags"
)

var opts struct {
	LogFormat string `long:"log-format" choice:"text" choice:"json" default:"text" description:"Log format"`
	Verbose   []bool `short:"v" long:"verbose" description:"Show verbose debug information, each -v bumps log level"`
	logLevel  slog.Level
	Sentinel  string `short:"s" long:"sentinel" default:".git" description:"Sentinel folder to stop searching"`
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
	scanner := bufio.NewScanner(os.Stdin)
	var paths []string

	for scanner.Scan() {
		paths = append(paths, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "Error reading input:", err)
		os.Exit(1)
	}

	var resultBuffer bytes.Buffer
	sentinelDirs := findSentinelDirs(paths)
	for _, dir := range sentinelDirs {
		resultBuffer.WriteString(dir)
		resultBuffer.WriteString("\n")
	}

	fmt.Print(resultBuffer.String())

	return nil
}

func findSentinelDirs(paths []string) []string {
	uniqueDirs := make(map[string]bool)
	var result []string

	for _, path := range paths {
		currentDir := filepath.Dir(path)

		for currentDir != "/" && !uniqueDirs[currentDir] {
			sentinelDir := filepath.Join(currentDir, opts.Sentinel)
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
