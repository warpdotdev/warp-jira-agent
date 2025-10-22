package cmd

import (
	"context"
	"errors"

	atlassian "github.com/ctreminiom/go-atlassian/v2/jira/v3"
	"github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var commentCmd = &cobra.Command{
	Use:   "comment <text>",
	Short: "Add a comment to a Jira issue",
	Long: `Add a comment to a Jira issue by providing the issue key and comment text.`,
	Example: `
  warp-jira-agent comment --issue PROJ-123 "This is my comment"
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := NewJiraClient()
		if err != nil {
			return err
		}

		issueKey := viper.GetString("issue")
		if issueKey == "" {
			return errors.New("issue key is required")
		}

		commentText := args[0]

		return addComment(cmd.Context(), client, issueKey, commentText)
	},
}

func init() {
	rootCmd.AddCommand(commentCmd)

	commentCmd.Flags().String("issue", "", "Jira issue key (e.g., PROJ-123)")
}

func addComment(ctx context.Context, client *atlassian.Client, issueKey, commentText string) error {
	log.Info().
		Str("issue", issueKey).
		Msg("adding comment to issue")

	payload := &models.CommentPayloadScheme{
		Body: &models.CommentNodeScheme{
			Version: 1,
			Type:    "doc",
			Content: []*models.CommentNodeScheme{
				{
					Type: "paragraph",
					Content: []*models.CommentNodeScheme{
						{
							Type: "text",
							Text: commentText,
						},
					},
				},
			},
		},
	}

	comment, response, err := client.Issue.Comment.Add(ctx, issueKey, payload, nil)
	if err != nil {
		if response != nil {
			log.Error().
				Err(err).
				Str("response.status", response.Status).
				Str("response.body", response.Bytes.String()).
				Msg("failed to add comment")
		}
		return err
	}
	response.Body.Close()

	log.Info().
		Str("issue", issueKey).
		Str("commentId", comment.ID).
		Msg("successfully added comment")

	return nil
}
