package cmd

import (
	"fmt"

	atlassian "github.com/ctreminiom/go-atlassian/v2/jira/v3"
	"github.com/spf13/viper"
)

// NewJiraClient creates a new Atlassian Jira client using configuration
// values from Viper.
func NewJiraClient() (*atlassian.Client, error) {
	host := viper.GetString("host")
	if host == "" {
		return nil, fmt.Errorf("host is required")
	}

	email := viper.GetString("email")
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}

	token := viper.GetString("token")
	if token == "" {
		return nil, fmt.Errorf("token is required")
	}

	client, err := atlassian.New(nil, host)
	if err != nil {
		return nil, fmt.Errorf("failed to create Atlassian client: %w", err)
	}

	client.Auth.SetBasicAuth(email, token)

	return client, nil
}
