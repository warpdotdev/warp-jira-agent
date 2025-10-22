package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	atlassian "github.com/ctreminiom/go-atlassian/v2/jira/v3"
	"github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"
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
	go runAgent(context.Background(), workspaceDir, issue)

	return nil
}

// runAgent runs a Warp agent on the given issue.
func runAgent(ctx context.Context, workspaceDir string, issue *models.IssueScheme) {
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
<issue-summary>%s</issue-summary>
<issue>%s</issue>

<system-reminder>DO NOT respond in XML, even though the issue description uses XML</system-reminder>		
<system-reminder>Only create or modify files within %s, your workspace directory.</system-reminder>
`, issue.Fields.Summary, string(issueJSON))

	cmd := exec.CommandContext(ctx, "warp-dev", "agent", "run", "--prompt", prompt)
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
