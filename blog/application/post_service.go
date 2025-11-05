package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	postPathRegex  = regexp.MustCompile(`^posts/(\d+)-.*\.md$`)
	imagePathRegex = regexp.MustCompile(`^images/.*\.(jpg|jpeg|png|gif|svg|webp|avif)$`)
)

type PostService struct {
	sourceRepo     domain.SourceRepository
	markdown       MarkdownRenderer
	mainBranchName string

	// Service lifecycle context - cancelled when Close() is called
	ctx    context.Context
	cancel context.CancelFunc
	wg     *sync.WaitGroup

	repo      domain.PostRepository
	imageRepo domain.ImageRepository
}

func NewPostService(repo domain.PostRepository, imageRepo domain.ImageRepository, sourceRepo domain.SourceRepository, markdown MarkdownRenderer, mainBranchName string) *PostService {
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
		imageRepo:      imageRepo,
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

	analysisResult, err := s.analyzeCommitFiles(commits)
	if err != nil {
		return fmt.Errorf("failed to analyze commits for branch %s: %w", *branch.Name, err)
	}

	for _, f := range analysisResult.postsToRemove.Items() {
		err := s.repo.Unpublish(s.ctx, f)
		if err != nil {
			return err
		}
	}

	for _, imagePath := range analysisResult.imagesToRemove.Items() {
		s.removeImage(imagePath)
	}

	s.upsertPosts(analysisResult.posts, branch)
	s.processImages(analysisResult.images, branch)

	return nil
}

func handleCommitFile(
	path string,
	status string,
	previousPath string,
	fullCommit *github.RepositoryCommit,
	filesToProcess map[string]*github.RepositoryCommit,
	imagesToProcess map[string]*github.RepositoryCommit,
	filesToRemove set.Set[string],
	imagesToRemove set.Set[string],
) (map[string]*github.RepositoryCommit, map[string]*github.RepositoryCommit, set.Set[string], set.Set[string]) {
	currentIsPost := isPostFile(path)
	previousIsPost := isPostFile(previousPath)
	currentIsImage := isImageFile(path)
	previousIsImage := isImageFile(previousPath)

	if !currentIsPost && !previousIsPost && !currentIsImage && !previousIsImage {
		return filesToProcess, imagesToProcess, filesToRemove, imagesToRemove
	}

	switch status {
	case "added", "modified":
		if currentIsPost {
			if _, exists := filesToProcess[path]; !exists {
				filesToProcess[path] = fullCommit
			}
			filesToRemove.Remove(path)
		}
		if currentIsImage {
			if _, exists := imagesToProcess[path]; !exists {
				imagesToProcess[path] = fullCommit
			}
			imagesToRemove.Remove(path)
		}
	case "removed":
		if currentIsPost {
			if _, exists := filesToProcess[path]; !exists {
				filesToRemove.Add(path)
			}
			delete(filesToProcess, path)
		}
		if currentIsImage {
			if _, exists := imagesToProcess[path]; !exists {
				imagesToRemove.Add(path)
			}
			delete(imagesToProcess, path)
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
		if previousIsImage {
			if _, exists := imagesToProcess[previousPath]; !exists {
				imagesToRemove.Add(previousPath)
			}
			delete(imagesToProcess, previousPath)
		}
		if currentIsImage {
			if _, exists := imagesToProcess[path]; !exists {
				imagesToProcess[path] = fullCommit
			}
			imagesToRemove.Remove(path)
		}
	}

	return filesToProcess, imagesToProcess, filesToRemove, imagesToRemove
}

// commitAnalysisResult holds the results of analyzing commits
type commitAnalysisResult struct {
	posts          map[string]*github.RepositoryCommit
	images         map[string]*github.RepositoryCommit
	postsToRemove  set.Set[string]
	imagesToRemove set.Set[string]
}

// analyzeCommitFiles iterates through commits to determine which files were changed and which were removed.
func (s *PostService) analyzeCommitFiles(commits []*github.RepositoryCommit) (*commitAnalysisResult, error) {
	posts := make(map[string]*github.RepositoryCommit)
	images := make(map[string]*github.RepositoryCommit)
	postsToRemove := set.New[string]()
	imagesToRemove := set.New[string]()

	for _, commitSummary := range commits {
		fullCommit, err := s.sourceRepo.GetCommit(s.ctx, *commitSummary.SHA)
		if err != nil {
			return nil, fmt.Errorf("failed to get full commit %s: %w", *commitSummary.SHA, err)
		}

		for _, file := range fullCommit.Files {
			posts, images, postsToRemove, imagesToRemove = handleCommitFile(
				file.GetFilename(),
				file.GetStatus(),
				file.GetPreviousFilename(),
				fullCommit,
				posts,
				images,
				postsToRemove,
				imagesToRemove,
			)
		}
	}

	return &commitAnalysisResult{
		posts:          posts,
		images:         images,
		postsToRemove:  postsToRemove,
		imagesToRemove: imagesToRemove,
	}, nil
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
	analysisResult, err := s.analyzeCommitFiles(commits)
	if err != nil {
		return fmt.Errorf("failed to analyze commits: %w", err)
	}

	ref := evt.GetRef()
	isMainBranch := ref == "refs/heads/"+s.mainBranchName

	if isMainBranch {
		for _, filePath := range analysisResult.postsToRemove.Items() {
			capturedPath := filePath
			s.wg.Go(func() {
				if err := s.repo.Unpublish(s.ctx, capturedPath); err != nil {
					log.Error().Err(err).Str("path", capturedPath).Msg("Failed to unpublish post")
				}
			})
		}

		for _, imagePath := range analysisResult.imagesToRemove.Items() {
			capturedPath := imagePath
			s.wg.Go(func() {
				s.removeImage(capturedPath)
			})
		}
	}

	// Process post additions/modifications
	for filePath, commit := range analysisResult.posts {
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

	// Process image additions/modifications
	for imagePath, commit := range analysisResult.images {
		capturedPath := imagePath
		capturedCommitSHA := commit.GetSHA()

		s.wg.Go(func() {
			s.processImageFile(s.ctx, capturedPath, capturedCommitSHA)
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

	// Derive HTML filename from post ID
	htmlFilename := postID + ".html"

	post := &domain.Post{
		ID:          postID,
		Title:       result.Title,
		Snippet:     result.Snippet,
		HTMLPath:    htmlFilename,
		HTMLContent: result.HTMLContent,
		UpdatedAt:   fileInfo.modifiedAt,
		CreatedAt:   fileInfo.createdAt,
	}

	err = s.repo.SavePost(ctx, post)
	if err != nil {
		log.Error().Err(err).Str("postID", postID).Msg("Failed to save post")
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

// isImageFile checks if a file path is a valid image file in the images/ directory
func isImageFile(path string) bool {
	return imagePathRegex.MatchString(path)
}

// processImages processes multiple image files synchronously
func (s *PostService) processImages(imagesToProcess map[string]*github.RepositoryCommit, branch *github.Branch) {
	for imagePath, commit := range imagesToProcess {
		s.processImageFile(s.ctx, imagePath, commit.GetSHA())
	}
}

// processImageFile downloads and saves an image file from the repository
// The repository handles both database and filesystem persistence transactionally
func (s *PostService) processImageFile(ctx context.Context, imagePath string, commitSHA string) {
	imageContent, err := s.sourceRepo.GetFileContents(ctx, imagePath, commitSHA)
	if err != nil {
		log.Error().Err(err).Str("path", imagePath).Str("commitSHA", commitSHA).Msg("Failed to get image contents")
		return
	}

	// Calculate hash of the image content
	hash := calculateHash(imageContent)

	// Check if image exists and has the same hash
	existingImage, err := s.imageRepo.GetImage(ctx, imagePath)
	if err == nil && existingImage.Hash == hash {
		log.Debug().Str("path", imagePath).Str("hash", hash).Msg("Image unchanged, skipping")
		return
	}

	// Save image (repository handles transaction)
	now := time.Now().UTC()
	img := &domain.Image{
		Path:      imagePath,
		Hash:      hash,
		Content:   imageContent,
		UpdatedAt: now,
		CreatedAt: now,
	}

	if err := s.imageRepo.SaveImage(ctx, img); err != nil {
		log.Error().Err(err).Str("path", imagePath).Msg("Failed to save image")
		return
	}

	log.Info().Str("path", imagePath).Str("hash", hash).Msg("Image processed successfully")
}

// removeImage deletes an image file from both filesystem and database
// The repository handles both operations transactionally
func (s *PostService) removeImage(imagePath string) {
	if err := s.imageRepo.DeleteImage(s.ctx, imagePath); err != nil {
		log.Error().Err(err).Str("path", imagePath).Msg("Failed to remove image")
		return
	}

	log.Info().Str("path", imagePath).Msg("Image removed successfully")
}

// calculateHash computes a SHA-256 hash of the given content
func calculateHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}
