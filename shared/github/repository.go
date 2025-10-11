package github

import (
	"context"
	"fmt"
	"time"

	"github.com/dfryer1193/goblog/blog/domain"
	"github.com/google/go-github/v75/github"
)

// GithubSourceRepository is an implementation of domain.SourceRepository that uses the GitHub API.
type GithubSourceRepository struct {
	client  *github.Client
	owner   string
	gitRepo string
}

// NewGithubSourceRepository creates a new GithubSourceRepository.
func NewGithubSourceRepository(client *github.Client, owner string, gitRepo string) domain.SourceRepository {
	return &GithubSourceRepository{
		client:  client,
		owner:   owner,
		gitRepo: gitRepo,
	}
}

// GetCommitsSince fetches commits for a branch since a given time.
func (g *GithubSourceRepository) GetCommitsSince(ctx context.Context, branchName string, since time.Time) ([]*github.RepositoryCommit, error) {
	commits, _, err := g.client.Repositories.ListCommits(ctx, g.owner, g.gitRepo, &github.CommitsListOptions{
		SHA:   branchName,
		Since: since,
	})
	if err != nil {
		return nil, fmt.Errorf("github: failed to list commits for branch %s: %w", branchName, err)
	}
	return commits, nil
}

// GetCommit fetches a single commit by its SHA.
func (g *GithubSourceRepository) GetCommit(ctx context.Context, sha string) (*github.RepositoryCommit, error) {
	commit, _, err := g.client.Repositories.GetCommit(ctx, g.owner, g.gitRepo, sha, nil)
	if err != nil {
		return nil, fmt.Errorf("github: failed to get commit %s: %w", sha, err)
	}
	return commit, nil
}

// ListBranches fetches all branches for the repository, handling pagination.
func (g *GithubSourceRepository) ListBranches(ctx context.Context) ([]*github.Branch, error) {
	var allBranches []*github.Branch
	opts := &github.BranchListOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for {
		branches, resp, err := g.client.Repositories.ListBranches(ctx, g.owner, g.gitRepo, opts)
		if err != nil {
			return nil, fmt.Errorf("github: failed to retrieve branches for %s/%s: %w", g.owner, g.gitRepo, err)
		}
		allBranches = append(allBranches, branches...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return allBranches, nil
}

// GetRepoFullName returns the repository's full name (e.g., "owner/repo").
func (g *GithubSourceRepository) GetRepoFullName() string {
	return fmt.Sprintf("%s/%s", g.owner, g.gitRepo)
}
