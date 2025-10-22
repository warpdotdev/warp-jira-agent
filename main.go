package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	atlassian "github.com/ctreminiom/go-atlassian/jira/v3"
	"github.com/ctreminiom/go-atlassian/pkg/infra/models"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const workspacesDir = "workspaces"

func main() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	log.Info().Msg("jira-bot starting")
	
	if err := godotenv.Load(); err != nil {
		log.Info().Msg(".env file not found, using existing environment variables")
	}

	// Read configuration from environment variables
	jiraHost := os.Getenv("JIRA_HOST")
	if jiraHost == "" {
		log.Fatal().Msg("JIRA_HOST environment variable is required")
	}

	jiraEmail := os.Getenv("JIRA_EMAIL")
	if jiraEmail == "" {
		log.Fatal().Msg("JIRA_EMAIL environment variable is required")
	}

	jiraToken := os.Getenv("JIRA_TOKEN")
	if jiraToken == "" {
		log.Fatal().Msg("JIRA_TOKEN environment variable is required")
	}
	
	label := os.Getenv("JIRA_LABEL")
	if label == "" {
		log.Fatal().Msg("JIRA_LABEL environment variable is required")
	}

	// Initialize go-atlassian client
	client, err := atlassian.New(nil, jiraHost)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create Atlassian client")
	}

	client.Auth.SetBasicAuth(jiraEmail, jiraToken)

	// Create workspaces directory if it doesn't exist
	if err := os.MkdirAll(workspacesDir, 0755); err != nil {
		log.Fatal().Err(err).Msg("failed to create workspaces directory")
	}

	// Start polling for new issues
	ctx := context.Background()
	pollForNewIssues(ctx, client, label)
}

func pollForNewIssues(ctx context.Context, client *atlassian.Client, label string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("polling stopped")
			return
		case <-ticker.C:
			if err := searchIssues(ctx, client, label, handleIssue); err != nil {
				log.Error().Err(err).Msg("failed to search issues")
			}
		}
	}
}

func handleIssue(issue *models.IssueScheme) error {
	workspaceDir := filepath.Join(workspacesDir, issue.Key)
	err := os.Mkdir(workspaceDir, 0755)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			log.Info().
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
		Str("summary", issue.Fields.Summary).
		Str("status", issue.Fields.Status.Name).
		Str("directory", workspaceDir).
		Msg("processing issue")
	return nil
}

// Search for issues with the label `label`, and run `callback` on each one.
func searchIssues(ctx context.Context, client *atlassian.Client, label string, callback func(*models.IssueScheme) error) error {
	jql := "labels = " + label + " ORDER BY created DESC"

	const pageSize = 50
	startAt := 0
	totalIssues := 0

	for {
		options := &models.SearchOptionsScheme{
			MaxResults: pageSize,
			StartAt:    startAt,
		}

		result, response, err := client.Issue.Search.Post(ctx, jql, nil, []string{"summary", "status"}, options)
		if err != nil {
			return err
		}
		response.Body.Close()

		for i := range result.Issues {
			if err := callback(&result.Issues[i]); err != nil {
				return err
			}
		}

		totalIssues += len(result.Issues)

		// Check if we've retrieved all issues
		if startAt+len(result.Issues) >= result.Total {
			break
		}

		startAt += pageSize
	}

	log.Info().
		Str("label", label).
		Int("total", totalIssues).
		Msg("finished processing issues")

	return nil
}
