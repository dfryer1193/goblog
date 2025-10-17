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
	"github.com/rs/zerolog/log"
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
func (s *PostService) SyncRepositoryChanges() error {
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
			log.Error().Err(err).Str("branch", *b.Name).Msg("Failed to process branch")
			continue
		}
	}

	return nil
}

func (s *PostService) processBranch(lastUpdatedAt time.Time, branch *github.Branch) error {
	commits, err := s.sourceRepo.GetCommitsSince(s.ctx, *branch.Name, lastUpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to get commits for branch %s: %w", *branch.Name, err)
	}

	if len(commits) == 0 {
		return nil
	}

	filesToProcess, filesToRemove, err := s.analyzeCommitFiles(commits)
	if err != nil {
		return fmt.Errorf("failed to analyze commits for branch %s: %w", *branch.Name, err)
	}

	for _, f := range filesToRemove.Items() {
		err := s.repo.Unpublish(s.ctx, f)
		if err != nil {
			return err
		}
	}

	s.upsertPosts(filesToProcess, branch)

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
func (s *PostService) upsertPosts(filesToProcess map[string]*github.RepositoryCommit, branch *github.Branch) error {
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
		capturedFileInfo := fileInfo
		// Use the commit SHA instead of ref to get the exact file version
		capturedCommitSHA := commit.GetSHA()

		s.processPostFile(
			s.ctx,
			capturedPostID,
			capturedFileInfo,
			capturedCommitSHA,
			isMainBranch,
		)
	}

	return nil
}

// HandlePushEvent processes a GitHub push event and updates posts accordingly
// This method returns immediately after validating the event and spawning async workers
// Workers use the service's lifecycle context, not the request context
func (s *PostService) HandlePushEvent(evt *github.PushEvent) error {
	// Get all commits in the push range
	var commits []*github.RepositoryCommit
	var err error

	if evt.GetBefore() != "" && evt.GetBefore() != "0000000000000000000000000000000000000000" {
		// Normal push with a base commit - get the range
		commits, err = s.sourceRepo.GetCommitsInRange(s.ctx, evt.GetBefore(), evt.GetAfter())
		if err != nil {
			return fmt.Errorf("failed to get commits in range %s...%s: %w", evt.GetBefore(), evt.GetAfter(), err)
		}
	} else {
		// New branch or first commit - just get the head commit
		headCommit, err := s.sourceRepo.GetCommit(s.ctx, evt.GetAfter())
		if err != nil {
			return fmt.Errorf("failed to get commit %s: %w", evt.GetAfter(), err)
		}
		commits = []*github.RepositoryCommit{headCommit}
	}

	// Analyze all commits to determine which files to process
	filesToProcess, filesToRemove, err := s.analyzeCommitFiles(commits)
	if err != nil {
		return fmt.Errorf("failed to analyze commits: %w", err)
	}

	ref := evt.GetRef()
	isMainBranch := ref == "refs/heads/"+s.mainBranchName

	if isMainBranch {
		for _, filePath := range filesToRemove.Items() {
			capturedPath := filePath
			s.wg.Go(func() {
				if err := s.repo.Unpublish(s.ctx, capturedPath); err != nil {
					log.Error().Err(err).Str("path", capturedPath).Msg("Failed to unpublish post")
				}
			})
		}
	}

	// Process additions/modifications
	for filePath, commit := range filesToProcess {
		postID := extractPostID(filePath)
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
			path:       filePath,
			createdAt:  createdAt,
			modifiedAt: modifiedAt,
		}

		// Capture variables for goroutine
		capturedPostID := postID
		capturedFileInfo := fileInfo
		// Use the commit SHA instead of ref to get the exact file version
		capturedCommitSHA := commit.GetSHA()

		s.wg.Go(func() {
			s.processPostFile(
				s.ctx,
				capturedPostID,
				capturedFileInfo,
				capturedCommitSHA,
				isMainBranch,
			)
		})
	}

	return nil
}

// processPostFile processes a single post file
// This function respects context cancellation for graceful shutdown
func (s *PostService) processPostFile(
	ctx context.Context,
	postID string,
	fileInfo commitFileInfo,
	commitSHA string,
	isMainBranch bool,
) {
	markdownContent, err := s.sourceRepo.GetFileContents(ctx, fileInfo.path, commitSHA)
	if err != nil {
		log.Error().Err(err).Str("path", fileInfo.path).Str("commitSHA", commitSHA).Msg("Failed to get file contents")
		return
	}

	result, err := s.markdown.Render(markdownContent)
	if err != nil {
		log.Error().Err(err).Str("path", fileInfo.path).Msg("Failed to render markdown")
		return
	}

	post := &domain.Post{
		ID:        postID,
		Title:     result.Title,
		Snippet:   result.Snippet,
		HTMLPath:  result.HTMLPath,
		UpdatedAt: fileInfo.modifiedAt,
		CreatedAt: fileInfo.createdAt,
	}

	err = s.repo.UpsertPost(ctx, post)
	if err != nil {
		log.Error().Err(err).Str("postID", postID).Msg("Failed to upsert post")
		return
	}

	if isMainBranch {
		err = s.repo.Publish(ctx, postID)
		if err != nil {
			log.Error().Err(err).Str("postID", postID).Msg("Failed to publish post")
			return
		}
	}
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
