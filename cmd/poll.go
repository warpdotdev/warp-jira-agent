package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	atlassian "github.com/ctreminiom/go-atlassian/v2/jira/v3"
	"github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

const workspacesDir = "workspaces"

type Repository struct {
	URL    string `yaml:"url"`
	Branch string `yaml:"branch"`
}

type ReposConfig struct {
	Repositories []Repository `yaml:"repositories"`
}

var pollCmd = &cobra.Command{
	Use:   "poll",
	Short: "Poll Jira for updates and dispatch agents",
	Long: `The polling driver searches Jira for issues with a particular label.

When a new issue is found, it dispatches a Warp agent to work on that issue.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := NewJiraClient()
		if err != nil {
			return err
		}

		label := viper.GetString("label")
		if label == "" {
			return errors.New("label is required")
		}

		// Load repository configuration at startup
		reposConfig, err := loadReposConfig()
		if err != nil {
			return err
		}

		return pollForNewIssues(cmd.Context(), client, label, reposConfig)
	},
}

func init() {
	rootCmd.AddCommand(pollCmd)

	pollCmd.Flags().String("label", "warp-assign", "Jira label to poll for")
}

func pollForNewIssues(ctx context.Context, client *atlassian.Client, label string, reposConfig *ReposConfig) error {
	// Create workspaces directory if it doesn't exist
	if err := os.MkdirAll(workspacesDir, 0755); err != nil {
		log.Error().Err(err).Msg("failed to create workspaces directory")
		return err
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("polling stopped")
			return nil
		case <-ticker.C:
			handler := func(issue *models.IssueScheme) error {
				return handleIssue(issue, reposConfig)
			}
			if err := searchIssues(ctx, client, label, handler); err != nil {
				log.Error().Err(err).Msg("failed to search issues")
			}
		}
	}

	return nil
}

func handleIssue(issue *models.IssueScheme, reposConfig *ReposConfig) error {
	workspaceDir := filepath.Join(workspacesDir, issue.Key)
	err := os.Mkdir(workspaceDir, 0755)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			log.Debug().
				Str("key", issue.Key).
				Msg("issue was already handled")
			return nil
		}
		log.Error().
			Err(err).
			Str("key", issue.Key).
			Str("directory", workspaceDir).
			Msg("failed to create workspace directory")
		return err
	}

	log.Info().
		Str("key", issue.Key).
		Msg("processing issue")

	// Run agent in a new goroutine
	go runAgent(context.Background(), workspaceDir, issue, reposConfig)

	return nil
}

// runAgent runs a Warp agent on the given issue.
func runAgent(ctx context.Context, workspaceDir string, issue *models.IssueScheme, reposConfig *ReposConfig) {
	outputLogPath := filepath.Join(workspaceDir, "output.log")
	logger := log.With().
		Str("key", issue.Key).
		Str("logPath", outputLogPath).
		Logger()

	logFile, err := os.Create(outputLogPath)
	if err != nil {
		logger.Error().
			Err(err).
			Msg("failed to create output log file")
		return
	}
	defer logFile.Close()

	// Setup repository worktrees
	if err := setupRepositoryWorktrees(ctx, workspaceDir, issue.Key, reposConfig, logger); err != nil {
		logger.Error().
			Err(err).
			Msg("failed to setup repository worktrees")
		return
	}

	// Atlassian uses a custom rich text JSON representation called ADF (the Atlassian Document Format).
	// See https://developer.atlassian.com/cloud/jira/platform/apis/document/structure/
	//
	// For simplicity, we marshal the JSON as-is and expect the agent to figure it out.
	// There are also a couple ADF libraries for Go that convert to HTML/Markdown:
	// - https://github.com/pinpt/adf
	// - https://pkg.go.dev/github.com/jcstorino/jira-cli/pkg/adf
	issueJSON, err := json.MarshalIndent(issue, "", "  ")
	if err != nil {
		logger.Error().
			Err(err).
			Msg("failed to marshal issue to JSON")
		return
	}

	// Format prompt with XML structure
	prompt := fmt.Sprintf(`
Address the following Jira issue to the best of your ability. You are given information about the issue in a simplified XML format.
<issue-key>%s</issue-key>
<issue-summary>%s</issue-summary>
<issue>%s</issue>

As you make progress on the issue, you can post comments by running this command:
<comment-command>
warp-jira-agent comment --issue {issue-key} "{comment-text}"
</comment-command>

<system-reminder>DO NOT respond in XML, even though the issue description uses XML</system-reminder>		
<system-reminder>Only create or modify files within %s, your workspace directory.</system-reminder>
`, issue.Key, issue.Fields.Summary, string(issueJSON), workspaceDir)

	// Build warp-cli arguments, optionally adding a profile ID
	args := []string{"agent", "run", "--prompt", prompt, "--debug"}
	if profileID := viper.GetString("profile_id"); profileID != "" {
		logger.Info().Str("profile_id", profileID).Msg("using warp profile for warp-cli")
		args = append(args, "--profile", profileID)
	}

	cmd := exec.CommandContext(ctx, "warp-cli-dev", args...)
	cmd.Dir = workspaceDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	logger.Info().Msg("starting warp agent")
	if err := cmd.Run(); err != nil {
		logger.Error().
			Err(err).
			Msg("warp agent command failed")
		return
	}

	logger.Info().Msg("warp agent completed")
}

// loadReposConfig loads the repository configuration from repos.yaml
func loadReposConfig() (*ReposConfig, error) {
	reposYAMLPath := "repos.yaml"
	data, err := os.ReadFile(reposYAMLPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Warn().Msg("repos.yaml not found, repository setup will be skipped")
			return &ReposConfig{}, nil
		}
		return nil, fmt.Errorf("failed to read repos.yaml: %w", err)
	}

	var config ReposConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse repos.yaml: %w", err)
	}

	log.Info().
		Int("count", len(config.Repositories)).
		Msg("loaded repository configuration")

	return &config, nil
}

// setupRepositoryWorktrees creates git worktrees for each repository
func setupRepositoryWorktrees(ctx context.Context, workspaceDir, issueKey string, reposConfig *ReposConfig, logger zerolog.Logger) error {
	if len(reposConfig.Repositories) == 0 {
		logger.Debug().Msg("no repositories configured, skipping worktree setup")
		return nil
	}

	// Create worktrees for each repository
	for _, repo := range reposConfig.Repositories {
		if err := createWorktree(ctx, workspaceDir, issueKey, repo, logger); err != nil {
			return fmt.Errorf("failed to create worktree for %s: %w", repo.URL, err)
		}
	}

	return nil
}

// createWorktree creates a git worktree for a repository
func createWorktree(ctx context.Context, workspaceDir, issueKey string, repo Repository, logger zerolog.Logger) error {
	// Extract repo name from URL (e.g., "warpdotdev/warp-server" -> "warp-server")
	parts := strings.SplitN(repo.URL, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid repository URL format: %s", repo.URL)
	}

	repoName := parts[1]
	repoDir := filepath.Join("repos", repoName)
	worktreeDir := filepath.Join(workspaceDir, repoName)

	// Convert to absolute paths
	absoluteRepoDir, err := filepath.Abs(repoDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for %s: %w", repoDir, err)
	}
	absoluteWorktreeDir, err := filepath.Abs(worktreeDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for %s: %w", worktreeDir, err)
	}

	// Create git worktree with branch named after issue key
	branchName := fmt.Sprintf("warp/%s-resolve", issueKey)
	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "-b", branchName, absoluteWorktreeDir, repo.Branch)
	cmd.Dir = absoluteRepoDir

	logger.Info().
		Str("repo", repoName).
		Str("branch", branchName).
		Str("worktreeDir", absoluteWorktreeDir).
		Msg("creating git worktree")

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create worktree: %w (output: %s)", err, string(output))
	}

	logger.Info().
		Str("repo", repoName).
		Str("worktreeDir", absoluteWorktreeDir).
		Msg("git worktree created successfully")

	return nil
}

// Search for issues with the label `label`, and run `callback` on each one.
func searchIssues(ctx context.Context, client *atlassian.Client, label string, callback func(*models.IssueScheme) error) error {
	log.Info().Str("label", label).Msg("Searching for Jira issues")

	jql := "labels = " + label + " ORDER BY created DESC"

	const pageSize = 50

	// Fields to return for each issue.
	fields := []string{"summary", "status", "id", "description"}

	// Expansions to apply on each issue.
	expands := []string{}

	var nextPageToken string
	for {
		result, response, err := client.Issue.Search.SearchJQL(ctx, jql, fields, expands, pageSize, nextPageToken)
		if err != nil {
			if response != nil {
				log.Error().
					Str("response.status", response.Status).
					Str("response.body", response.Bytes.String()).
					Msg("jira search failed")
			}

			return err
		}
		response.Body.Close()

		for i := range result.Issues {
			if err := callback(result.Issues[i]); err != nil {
				return err
			}
		}

		if result.NextPageToken == "" {
			break
		} else {
			nextPageToken = result.NextPageToken
		}
	}

	log.Info().
		Str("label", label).
		Msg("Finished searching for Jira issues")

	return nil
}
