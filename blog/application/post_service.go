package application

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/dfryer1193/goblog/blog/domain"
	"github.com/dfryer1193/mjolnir/utils/set"
	"github.com/google/go-github/v75/github"
)

type PostService struct {
	// TODO: Add dependencies like PostRepository, GitHub client, etc.
	githubClient *github.Client

	// Service lifecycle context - cancelled when Close() is called
	ctx    context.Context
	cancel context.CancelFunc
	wg     *sync.WaitGroup

	repo domain.PostRepository
}

func NewPostService(repo domain.PostRepository) *PostService {
	authToken := os.Getenv("GITHUB_TOKEN")
	ghClient := github.NewClient(nil)
	if authToken != "" {
		ghClient = ghClient.WithAuthToken(authToken)
	}

	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}
	return &PostService{
		githubClient: ghClient,
		ctx:          ctx,
		cancel:       cancel,
		wg:           &wg,
		repo:         repo,
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
func (s *PostService) SyncRepositoryChanges(ctx context.Context, owner, repo string, since time.Time) error {
	lastUpdated, err := s.repo.GetLatestUpdatedTime()
	if err != nil {
		return err
	}

	// GET /repos/{owner}/{repo}/branches
	branches, resp, err := s.githubClient.Repositories.ListBranches(ctx, owner, repo, &github.BranchListOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	})
	if err != nil {
		return fmt.Errorf("failed to list branches: %w", err)
	}

	for _, branch := range branches {
		branchName := branch.GetName()
		isMainBranch := branchName == "main" || branchName == "master"

		// Get commits since the last sync time
		// GET /repos/{owner}/{repo}/commits?sha={branch}&since={since}&path=posts
		commits, _, err := s.githubClient.Repositories.ListCommits(ctx, owner, repo, &github.CommitsListOptions{
			SHA:         branchName,
			Since:       since,
			Path:        "posts",
			ListOptions: github.ListOptions{PerPage: 100},
		})
		if err != nil {
			return fmt.Errorf("failed to list commits for branch %s: %w", branchName, err)
		}

		if len(commits) == 0 {
			continue // No changes in this branch
		}

		for _, commit := range commits {
			commitSHA := commit.GetSHA()
			commitTime := commit.GetCommit().GetCommitter().GetDate().Time

			// Get the commit details to see which files were changed
			// GET /repos/{owner}/{repo}/commits/{sha}
			commitDetail, _, err := s.githubClient.Repositories.GetCommit(ctx, owner, repo, commitSHA, nil)
			if err != nil {
				return fmt.Errorf("failed to get commit %s: %w", commitSHA, err)
			}

			// Track changed and deleted files in this commit
			changedFiles := set.New[string]()
			deletedFiles := set.New[string]()

			for _, file := range commitDetail.Files {
				filePath := file.GetFilename()
				if !isPostFile(filePath) {
					continue
				}

				status := file.GetStatus()
				if status == "removed" {
					deletedFiles.Add(filePath)
				} else {
					// "added", "modified", "renamed", etc.
					changedFiles.Add(filePath)
				}
			}

			// Process deleted files - unset PublishedAt
			for filePath := range deletedFiles {
				postID := extractPostID(filePath)
				if postID == "" {
					continue
				}

				// TODO: Fetch existing post and unset PublishedAt
				// post, err := s.repo.GetPost(postID)
				// if err != nil {
				//     continue // Post doesn't exist, nothing to do
				// }
				// post.PublishedAt = time.Time{} // Zero value = unpublished
				// post.UpdatedAt = commitTime
				// err = s.repo.UpsertPost(post)
				// if err != nil {
				//     return fmt.Errorf("failed to unpublish post %s: %w", postID, err)
				// }
			}

			// Process changed files
			for filePath := range changedFiles {
				postID := extractPostID(filePath)
				if postID == "" {
					continue
				}

				// TODO: Fetch the file content from the repository
				// content, err := s.fetchFileContent(ctx, owner, repo, filePath, branchName)
				// if err != nil {
				//     return fmt.Errorf("failed to fetch file %s: %w", filePath, err)
				// }

				// TODO: Parse markdown and extract title, snippet
				// title := extractTitle(content, filePath)
				// snippet := extractSnippet(content)

				// TODO: Convert markdown to HTML
				// htmlContent := convertMarkdownToHTML(content)

				// TODO: Store HTML content on filesystem
				// htmlPath := storeHTMLToFile(postID, htmlContent)

				// TODO: Get or create post
				// post, err := s.repo.GetPost(postID)
				// if err != nil {
				//     // Post doesn't exist, create new one
				//     post = &domain.Post{
				//         ID:        postID,
				//         CreatedAt: commitTime,
				//     }
				// }
				//
				// // Update post fields
				// post.Title = title
				// post.Snippet = snippet
				// post.HTMLPath = htmlPath
				// post.UpdatedAt = commitTime
				//
				// // Set PublishedAt only if this is the main branch
				// if isMainBranch {
				//     post.PublishedAt = commitTime
				// }
				//
				// err = s.repo.UpsertPost(post)
				// if err != nil {
				//     return fmt.Errorf("failed to upsert post %s: %w", postID, err)
				// }
			}
		}
	}

	return nil
}

func (s *PostService) UpsertPost() error {
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
	isMainBranch := ref == "refs/heads/main" || ref == "refs/heads/master"

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
				// Only set the creation time if this is the first time we see this file
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
		capturedCommitTime := latestCommitTime

		wg.Go(func() {
			// Use the service's lifecycle context for cancellation
			// TODO: Fetch existing post and unset PublishedAt
			// post, err := s.repo.GetPost(capturedPostID)
			// if err != nil {
			//     // Post doesn't exist, nothing to do
			//     return
			// }
			// post.PublishedAt = time.Time{} // Zero value = unpublished
			// post.UpdatedAt = capturedCommitTime
			// err = s.repo.UpsertPost(post)
			// if err != nil {
			//     // Log error but don't fail the whole batch
			//     log.Error().Err(err).Str("postID", capturedPostID).Msg("Failed to unpublish post")
			// }

			_ = capturedPostID
			_ = capturedCommitTime
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

// processPostFile processes a single post file asynchronously
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
	// Check if context is cancelled before starting expensive operations
	select {
	case <-ctx.Done():
		// TODO: Log cancellation
		// log.Debug().Str("postID", postID).Msg("Post processing cancelled")
		return
	default:
	}

	// TODO: Fetch the file content from the repository
	// This requires making a GitHub API call to get the file contents
	// content, err := s.fetchFileContent(ctx, repoFullName, ref, filePath)
	// if err != nil {
	//     if ctx.Err() != nil {
	//         // Context was cancelled
	//         return
	//     }
	//     // Log error but don't fail
	//     log.Error().Err(err).Str("filePath", filePath).Msg("Failed to fetch file content")
	//     return
	// }

	// Check cancellation again before expensive parsing
	select {
	case <-ctx.Done():
		return
	default:
	}

	// TODO: Parse markdown and extract title
	// Title should be the text of the first H1 header.
	// If there are no H1 headers, use the title from the filename, converted to title case.
	// title := extractTitle(content, filePath)

	// TODO: Extract snippet from markdown
	// snippet := extractSnippet(content)

	// TODO: Convert markdown to HTML (potentially expensive operation)
	// htmlContent := convertMarkdownToHTML(content)

	// Check cancellation before I/O operations
	select {
	case <-ctx.Done():
		return
	default:
	}

	// TODO: Store HTML content on filesystem
	// htmlPath := storeHTMLToFile(postID, htmlContent)

	// TODO: Create/update post in database
	// post := &domain.Post{
	//     ID:          postID,
	//     Title:       title,
	//     Snippet:     snippet,
	//     HTMLPath:    htmlPath,
	//     UpdatedAt:   fileInfo.modifiedAt,
	//     CreatedAt:   fileInfo.createdAt,
	// }
	//
	// if isMainBranch {
	//     post.PublishedAt = latestCommitTime
	// } else {
	//     // Non-main branch: process and store but don't publish
	//     post.PublishedAt = time.Time{} // Zero value = unpublished
	// }
	//
	// err = s.repo.UpsertPost(post)
	// if err != nil {
	//     if ctx.Err() != nil {
	//         // Context was cancelled
	//         return
	//     }
	//     // Log error but don't fail
	//     log.Error().Err(err).Str("postID", postID).Msg("Failed to upsert post")
	//     return
	// }

	_ = postID
	_ = filePath
	_ = fileInfo
	_ = ref
	_ = repoFullName
	_ = isMainBranch
	_ = latestCommitTime
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
	// Check if file is in posts/ directory and ends with .md
	if len(path) < 10 || path[:6] != "posts/" || path[len(path)-3:] != ".md" {
		return false
	}

	// Extract filename from path
	filename := path[6:] // Remove "posts/" prefix

	// Check if filename starts with digits followed by a hyphen
	digitCount := 0
	for _, ch := range filename {
		if ch >= '0' && ch <= '9' {
			digitCount++
		} else if ch == '-' && digitCount > 0 {
			// Valid format: at least one digit followed by hyphen
			return true
		} else {
			// Invalid format
			return false
		}
	}

	return false
}

// extractPostID extracts the numeric ID from a post filename
// Example: "posts/001-my-post.md" -> "001"
func extractPostID(path string) string {
	if len(path) < 10 || path[:6] != "posts/" {
		return ""
	}

	filename := path[6:] // Remove "posts/" prefix

	// Extract digits before the first hyphen
	id := ""
	for _, ch := range filename {
		if ch >= '0' && ch <= '9' {
			id += string(ch)
		} else if ch == '-' {
			break
		} else {
			return ""
		}
	}

	return id
}
