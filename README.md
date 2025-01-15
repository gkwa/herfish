# herfish

herfish is a command-line utility that finds Git repository root directories by traversing upward from given file paths to locate their parent .git directories.

## Overview

herfish takes file paths from stdin and for each path, searches upward through the directory tree to find the nearest parent directory containing a .git folder. This makes it easy to identify the root of any Git repository by finding its parent .git directory. It then outputs the path to the directory containing that .git folder.

## Usage

herfish reads paths from standard input, making it ideal for use in pipelines:

```bash
# Find Go files containing specific content and get their Git root directories
mdfind 'kMDItemFSName=*.go && kMDItemTextContent=="Walk" && kMDItemTextContent==".git"' | herfish
```

## Example

Input:
```bash
/path/to/project1/src/main.go
/path/to/project2/core/handler.go
/path/to/project3/internal/core.go
```

Output:
```bash
/path/to/project1
/path/to/project2
/path/to/project3
```

## Features

Each input path is processed independently.

The tool returns unique Git root directories.

Output paths are sorted alphabetically.

## Common Use Cases

Finding Git repositories in complex directory structures:
```bash
# Find all Python files and get their Git root directories
find . -name "*.py" | herfish

# Search for specific content and find associated Git repositories
grep -l "TODO" **/*.go | herfish
```

## System Requirements

Requires a Unix-like environment with standard filesystem operations.

Git must be installed on the system.

## Installation

```bash
go install github.com/taylormonacelli/herfish@latest
```

## Bash Equivalent

herfish's functionality can be replicated using this bash command:

```bash
while [[ "$PWD" != "/" ]]; do
    if [[ -d ".git" ]]; then
        echo "$PWD"
        break
    fi
    cd ..
done
```

However, herfish provides this functionality in a more convenient form that:
- Handles multiple paths in parallel
- Doesn't require changing the current directory
- Returns clean, sorted output
- Can be easily integrated into pipelines

## Quick Start

1. Install herfish using go install

2. Pipe any list of paths to herfish:
```bash
find . -type f | herfish
```

3. Review the output showing Git root directories