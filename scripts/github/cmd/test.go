/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/google/go-github/v54/github"
	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Test GitHub PAT",
	Long:  "Test GitHub PAT is valid",
	Run: func(cmd *cobra.Command, args []string) {
		token, _ := cmd.Flags().GetString("token")
		repo, _ := cmd.Flags().GetString("repo")

		err := test(repo, token)
		if err != nil {
			fmt.Println("error: ", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(testCmd)

	testCmd.Flags().StringP("token", "t", "", "GitHub API token")
	testCmd.Flags().StringP("repo", "r", "", "GitHub repository (Saza-ku/isucon13q)")
}

func test(repo string, token string) error {
	client := getClient(token)

	repo, name, err := parseRepo(repo)
	if err != nil {
		return err
	}

	err = createTestIssue(client, repo, name)
	if err != nil {
		return err
	}

	log.Println("Successfully created a test issue")

	err = createTestComment(client, repo, name)
	if err != nil {
		return err
	}

	log.Println("Successfully created a test comment")

	return nil
}

func createTestIssue(client *github.Client, repo string, name string) error {
	err := createMeasureLabelIfNotExists(client, repo, name)
	if err != nil {
		return err
	}

	_, _, err = client.Issues.Create(context.Background(), repo, name, &github.IssueRequest{
		Title: github.String("Test Issue"),
		Body:  github.String("This is a test issue"),
		Labels: &[]string{
			"measure",
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func createTestComment(client *github.Client, repo string, name string) error {
	issue, err := getLatestIssue(client, repo, name)
	if err != nil {
		return err
	}

	_, _, err = client.Issues.CreateComment(context.Background(), repo, name, *issue.Number, &github.IssueComment{
		Body: github.String("This is a test comment"),
	})
	if err != nil {
		return err
	}

	return nil
}
