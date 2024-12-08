package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v54/github"
	"golang.org/x/oauth2"
)

func getClient(token string) *github.Client {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	return github.NewClient(tc)
}

func parseRepo(repo string) (string, string, error) {
	splitted := strings.Split(repo, "/")
	if len(splitted) != 2 {
		return "", "", fmt.Errorf("invalid repo: %s", repo)
	}

	return splitted[0], splitted[1], nil
}
