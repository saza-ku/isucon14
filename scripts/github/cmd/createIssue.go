/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/google/go-github/v54/github"
	"github.com/spf13/cobra"
)

// createIssueCmd represents the createIssue command
var createIssueCmd = &cobra.Command{
	Use:   "create-issue",
	Short: "Create Issue",
	Long:  "Create Issue",
	Run: func(cmd *cobra.Command, args []string) {

		token, _ := cmd.Flags().GetString("token")
		repo, _ := cmd.Flags().GetString("repo")
		date, _ := cmd.Flags().GetString("date")

		err := createIssue(token, repo, date)

		if err != nil {
			fmt.Println("error: ", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(createIssueCmd)

	createIssueCmd.Flags().StringP("token", "t", "", "GitHub API token")
	createIssueCmd.Flags().StringP("repo", "r", "", "GitHub repository (Saza-ku/isucon13q)")
	createIssueCmd.Flags().StringP("date", "d", "", "Date (10121200)")
}

func createIssue(token string, repo string, date string) error {
	client := getClient(token)

	repo, name, err := parseRepo(repo)
	if err != nil {
		return err
	}

	// create measure label if not exists
	err = createMeasureLabelIfNotExists(client, repo, name)
	if err != nil {
		return err
	}

	// create issue
	_, _, err = client.Issues.Create(context.Background(), repo, name, &github.IssueRequest{
		Title: github.String(date),
		Body:  github.String(""),
		Labels: &[]string{
			"measure",
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func createMeasureLabelIfNotExists(client *github.Client, owner string, repo string) error {
	labels, _, err := client.Issues.ListLabels(context.Background(), owner, repo, nil)
	if err != nil {
		return err
	}

	for _, label := range labels {
		if *label.Name == "measure" {
			return nil
		}
	}

	_, _, err = client.Issues.CreateLabel(context.Background(), owner, repo, &github.Label{
		Name:        github.String("measure"),
		Description: github.String("計測結果"),
		Color:       github.String("019167"),
	})
	if err != nil {
		return err
	}

	return nil
}
