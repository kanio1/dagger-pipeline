package main

import (
	"context"
	"fmt"
	"strings"

	"dagger/my-module/internal/dagger"
)

// MyModule provides git and linting functions.
type MyModule struct{}

// CommitInfo contains information about a git commit.
type CommitInfo struct {
	Hash    string
	Author  string
	Message string
}

// Clone clones a git repository from a given URL and reference, and returns a *dagger.Directory.
// This function is useful for starting a CI/CD pipeline with the source code.
// Note that this returns a directory snapshot, not a full git repository object.
func (m *MyModule) Clone(ctx context.Context, url string, ref string) (*dagger.Directory, error) {
	return dag.Git(url).Ref(ref).Tree(), nil
}

// GetCommitInfo retrieves commit information (hash, author, message) from a git commit object.
// This is the correct, idiomatic way to get commit information in Dagger.
// To get a commit object, you can use `dag.Git(url).Ref(ref).Commit()`.
func (m *MyModule) GetCommitInfo(ctx context.Context, commit *dagger.GitCommit) (*CommitInfo, error) {
	hash, err := commit.ID(ctx)
	if err != nil {
		return nil, err
	}

	author, err := commit.Author(ctx).Name(ctx)
	if err != nil {
		return nil, err
	}

	message, err := commit.Message(ctx)
	if err != nil {
		return nil, err
	}

	return &CommitInfo{
		Hash:    hash,
		Author:  author,
		Message: strings.TrimSpace(message),
	}, nil
}

// Lint runs a specified linter on the given source code.
// This function makes it easy to add a linting step to a pipeline.
// It supports "golangci-lint" and "eslint" as examples.
func (m *MyModule) Lint(ctx context.Context, source *dagger.Directory, linter string) (string, error) {
	var linterContainer *dagger.Container
	var execArgs []string

	switch linter {
	case "golangci-lint":
		linterContainer = dag.Container().From("golangci/golangci-lint:v1.55.2-alpine")
		execArgs = []string{"golangci-lint", "run", "./..."}
	case "eslint":
		// For linters that need to be installed, you can do it in the container.
		linterContainer = dag.Container().From("node:18-alpine").WithExec([]string{"npm", "install", "-g", "eslint"})
		execArgs = []string{"eslint", "."}
	default:
		return "", fmt.Errorf("unsupported linter: %s. Supported linters are 'golangci-lint' and 'eslint'", linter)
	}

	// Mount the source code, run the linter, and return the output.
	result := linterContainer.WithMountedDirectory("/src", source).WithWorkdir("/src").WithExec(execArgs)

	return result.Stdout(ctx)
}

// CommitAndPush commits and pushes changes to a git repository.
// This is useful for pipelines that modify code, like auto-formatters or dependency updaters.
// It requires a git token with push access to be provided as a secret.
func (m *MyModule) CommitAndPush(
	ctx context.Context,
	source *dagger.Directory,
	remoteURL string,
	branch string,
	message string,
	token *dagger.Secret,
) (string, error) {
	// Use a container with git installed, and provide the token as a secret.
	gitContainer := dag.Container().
		From("alpine/git").
		WithSecretVariable("GIT_TOKEN", token).
		WithMountedDirectory("/src", source).
		WithExec([]string{"git", "config", "--global", "user.name", "Dagger CI"}).
		WithExec([]string{"git", "config", "--global", "user.email", "ci@dagger.io"}).
		WithExec([]string{
			"sh",
			"-c",
			fmt.Sprintf("git clone https://oauth2:${GIT_TOKEN}@%s /repo", remoteURL),
		}).
		WithWorkdir("/repo").
		WithExec([]string{"cp", "-a", "/src/.", "."}).
		WithExec([]string{"git", "add", "."}).
		WithExec([]string{"git", "commit", "-m", message}).
		WithExec([]string{"git", "push", "origin", branch})

	return gitContainer.Stdout(ctx)
}
