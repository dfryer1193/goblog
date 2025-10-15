package application

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/dfryer1193/goblog/blog/domain"
	"github.com/dfryer1193/mjolnir/utils/set"
	"github.com/google/go-github/v75/github"
)

var (
	postPathRegex = regexp.MustCompile(`^posts/(\d+)-.*\.md$`)
)

type PostService struct {
	sourceRepo     domain.SourceRepository
	markdown       MarkdownRenderer
	mainBranchName string

	// Service lifecycle context - cancelled when Close() is called
	ctx    context.Context
	cancel context.CancelFunc
	wg     *sync.WaitGroup

	repo domain.PostRepository
}

func NewPostService(repo domain.PostRepository, sourceRepo domain.SourceRepository, markdown MarkdownRenderer, mainBranchName string) *PostService {
	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}
	return &PostService{
		sourceRepo:     sourceRepo,
		markdown:       markdown,
		mainBranchName: mainBranchName,
		ctx:            ctx,
		cancel:         cancel,
		wg:             &wg,
		repo:           repo,
	}
}

// Close gracefully shuts down the PostService by cancelling all background workers
func (s *PostService) Close() error {
	s.cancel()
	s.wg.Wait()

	return nil
}

// SyncRepositoryChanges syncs posts from recent commits across all branches
// This catches any changes that happened while the server was offline
func (s *PostService) SyncRepositoryChanges(since time.Time) error {
	lastUpdatedAt, err := s.repo.GetLatestUpdatedTime(s.ctx)
	if err != nil {
		return fmt.Errorf("could not get the time of the last update: %w", err)
	}

	branches, err := s.sourceRepo.ListBranches(s.ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve branches: %w", err)
	}

	// don't worry about rate limits for the moment; we shouldn't be making calls in enough volume for it to be a problem.
	for _, branch := range branches {
		s.processBranches(lastUpdatedAt, []*github.Branch{branch})
	}

	return nil
}

func (s *PostService) processBranches(lastUpdatedAt time.Time, branches []*github.Branch) error {
	for _, b := range branches {
		err := s.processBranch(lastUpdatedAt, b)
		if err != nil {
			// TODO: use zerolog to log error and continue to next branch
			return err
		}
	}

	return nil
}

func (s *PostService) processBranch(lastUpdatedAt time.Time, branch *github.Branch) error {
	commits, err := s.sourceRepo.GetCommitsSince(s.ctx, *branch.Name, lastUpdatedAt)
	if err != nil {
		// TODO: use zerolog for error logging
		return fmt.Errorf("failed to get commits for branch %s: %w", *branch.Name, err)
	}

	if len(commits) == 0 {
		return nil
	}

	filesToProcess, filesToRemove, err := s.analyzeCommitFiles(commits)
	if err != nil {
		// TODO: use zerolog for error logging
		return fmt.Errorf("failed to analyze commits for branch %s: %w", *branch.Name, err)
	}

	for _, f := range filesToRemove.Items() {
		err := s.repo.Unpublish(s.ctx, f)
		if err != nil {
			return err
		}
	}

	latestCommitTime := commits[0].GetCommit().GetAuthor().GetDate().Time
	s.dispatchPostUpserts(filesToProcess, branch, latestCommitTime)

	return nil
}

func handleCommitFile(
	path string,
	status string,
	previousPath string,
	fullCommit *github.RepositoryCommit,
	filesToProcess map[string]*github.RepositoryCommit,
	filesToRemove set.Set[string],
) (map[string]*github.RepositoryCommit, set.Set[string]) {
	currentIsPost := isPostFile(path)
	previousIsPost := isPostFile(previousPath)

	if !currentIsPost && !previousIsPost {
		return filesToProcess, filesToRemove
	}

	switch status {
	case "added", "modified":
		if currentIsPost {
			if _, exists := filesToProcess[path]; !exists {
				filesToProcess[path] = fullCommit
			}
			filesToRemove.Remove(path)
		}
	case "removed":
		if currentIsPost {
			if _, exists := filesToProcess[path]; !exists {
				filesToRemove.Add(path)
			}
			delete(filesToProcess, path)
		}
	case "renamed":
		if previousIsPost {
			if _, exists := filesToProcess[previousPath]; !exists {
				filesToRemove.Add(previousPath)
			}
			delete(filesToProcess, previousPath)
		}
		if currentIsPost {
			if _, exists := filesToProcess[path]; !exists {
				filesToProcess[path] = fullCommit
			}
			filesToRemove.Remove(previousPath)
		}
	}

	return filesToProcess, filesToRemove
}

// analyzeCommitFiles iterates through commits to determine which files were changed and which were removed.
func (s *PostService) analyzeCommitFiles(commits []*github.RepositoryCommit) (map[string]*github.RepositoryCommit, set.Set[string], error) {
	filesToProcess := make(map[string]*github.RepositoryCommit)
	filesToRemove := set.New[string]()

	for _, commitSummary := range commits {
		fullCommit, err := s.sourceRepo.GetCommit(s.ctx, *commitSummary.SHA)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get full commit %s: %w", *commitSummary.SHA, err)
		}

		for _, file := range fullCommit.Files {
			filesToProcess, filesToRemove = handleCommitFile(file.GetFilename(), file.GetStatus(), file.GetPreviousFilename(), fullCommit, filesToProcess, filesToRemove)
		}
	}
	return filesToProcess, filesToRemove, nil
}

// upsertPosts processes and upserts posts from the given filesToProcess map
func (s *PostService) upsertPosts(filesToProcess map[string]*github.RepositoryCommit, branch *github.Branch, latestCommitTime time.Time) error {
	repoFullName := s.sourceRepo.GetRepoFullName()
	ref := "refs/heads/" + *branch.Name
	isMainBranch := ref == "refs/heads/"+s.mainBranchName

	for path, commit := range filesToProcess {
		postID := extractPostID(path)
		if postID == "" {
			continue
		}

		modifiedAt := commit.GetCommit().GetAuthor().GetDate().Time

		existingPost, err := s.repo.GetPost(s.ctx, postID)
		createdAt := modifiedAt
		if err == nil && existingPost != nil {
			createdAt = existingPost.CreatedAt
		}

		fileInfo := commitFileInfo{
			path:       path,
			createdAt:  createdAt,
			modifiedAt: modifiedAt,
		}

		capturedPostID := postID
		capturedPath := path
		capturedFileInfo := fileInfo

		s.processPostFile(
			s.ctx,
			capturedPostID,
			capturedPath,
			capturedFileInfo,
			ref,
			repoFullName,
			isMainBranch,
			latestCommitTime,
		)
	}

	return nil
}

// HandlePushEvent processes a GitHub push event and updates posts accordingly
// This method returns immediately after validating the event and spawning async workers
// Workers use the service's lifecycle context, not the request context
func (s *PostService) HandlePushEvent(evt *github.PushEvent) error {
	// TODO: Get expected repository name from environment variable or config
	expectedRepo := "" // TODO: Load from config/env
	if expectedRepo != "" && evt.GetRepo().GetFullName() != expectedRepo {
		// Not the repo we're interested in, silently ignore
		return nil
	}

	// Get the ref (branch) being pushed to
	ref := evt.GetRef()
	isMainBranch := ref == "refs/heads/"+s.mainBranchName

	// Process all commits in the push to catch any changes to posts
	commits := evt.GetCommits()
	if len(commits) == 0 {
		// No commits, nothing to process
		return nil
	}

	// Track all changed and deleted post files across all commits
	// Map from file path to the earliest commit time that introduced/modified it
	changedFiles := make(map[string]commitFileInfo)
	deletedFiles := make(map[string]bool)

	// Process commits in order to track file changes
	for _, commit := range commits {
		commitTime := commit.GetTimestamp().Time

		// Track added files (for CreatedAt timestamp)
		for _, file := range commit.Added {
			if isPostFile(file) {
				if _, exists := changedFiles[file]; !exists {
					changedFiles[file] = commitFileInfo{
						path:       file,
						createdAt:  commitTime,
						modifiedAt: commitTime,
					}
				}
				// Remove from deleted files if it was previously deleted
				delete(deletedFiles, file)
			}
		}

		// Track modified files
		for _, file := range commit.Modified {
			if isPostFile(file) {
				if info, exists := changedFiles[file]; exists {
					// Update the modified time but keep the original created time
					info.modifiedAt = commitTime
					changedFiles[file] = info
				} else {
					// File was modified but we didn't see it added in this push
					// Use the commit time for both created and modified
					changedFiles[file] = commitFileInfo{
						path:       file,
						createdAt:  commitTime,
						modifiedAt: commitTime,
					}
				}
				// Remove from deleted files if it was previously deleted
				delete(deletedFiles, file)
			}
		}

		// Track deleted files
		for _, file := range commit.Removed {
			if isPostFile(file) {
				deletedFiles[file] = true
				// Remove from changed files if it was previously changed
				delete(changedFiles, file)
			}
		}
	}

	// Get the latest commit time for PublishedAt field
	latestCommit := commits[len(commits)-1]
	latestCommitTime := latestCommit.GetTimestamp().Time

	// Spawn async workers to process posts using sync.WaitGroup.Go (Go 1.25+)
	// We don't wait for them to complete - they run in the background
	var wg sync.WaitGroup

	// Process deleted post files - unset PublishedAt
	for filePath := range deletedFiles {
		postID := extractPostID(filePath)
		if postID == "" {
			// Invalid post file format, skip silently
			continue
		}

		// Capture variables for closure
		capturedPostID := postID
		wg.Go(func() {
			// Use the service's lifecycle context for cancellation
			// TODO: Log error
			s.repo.Unpublish(s.ctx, capturedPostID)
		})
	}

	// Process changed post files
	for filePath, fileInfo := range changedFiles {
		postID := extractPostID(filePath)
		if postID == "" {
			// Invalid post file format, skip silently
			continue
		}

		// Capture variables for closure
		capturedPostID := postID
		capturedFilePath := filePath
		capturedFileInfo := fileInfo
		capturedRef := ref
		capturedIsMainBranch := isMainBranch
		capturedCommitTime := latestCommitTime
		capturedRepoFullName := evt.GetRepo().GetFullName()

		wg.Go(func() {
			// Use the service's lifecycle context for cancellation
			s.processPostFile(s.ctx, capturedPostID, capturedFilePath, capturedFileInfo, capturedRef, capturedRepoFullName, capturedIsMainBranch, capturedCommitTime)
		})
	}

	// Don't wait for the workers to complete - return immediately
	// The workers will continue processing in the background
	// Note: We don't need to explicitly wait - wg.Go handles the lifecycle
	// The goroutines will complete on their own, and errors are logged internally

	return nil
}

// processPostFile processes a single post file
// This function respects context cancellation for graceful shutdown
func (s *PostService) processPostFile(
	ctx context.Context,
	postID string,
	filePath string,
	fileInfo commitFileInfo,
	ref string,
	repoFullName string,
	isMainBranch bool,
	latestCommitTime time.Time,
) {
	// TODO: 1. run the file through the markdown processor
	// TODO: 2. save the post info to the database
	// TODO: 3. save the file to disk/storage
}

// commitFileInfo tracks when a file was first created and last modified in a push
type commitFileInfo struct {
	path       string
	createdAt  time.Time
	modifiedAt time.Time
}

// isPostFile checks if a file path is a valid post file in the posts/ directory
// Valid format: posts/NNN-title-of-post.md where NNN is one or more digits
func isPostFile(path string) bool {
	return postPathRegex.MatchString(path)
}

// extractPostID extracts the numeric ID from a post filename
// Example: "posts/001-my-post.md" -> "001"
func extractPostID(path string) string {
	matches := postPathRegex.FindStringSubmatch(path)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}
