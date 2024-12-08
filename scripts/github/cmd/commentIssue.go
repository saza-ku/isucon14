/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"text/template"

	"github.com/google/go-github/v54/github"
	"github.com/spf13/cobra"
)

// commentIssueCmd represents the commentIssue command
var commentIssueCmd = &cobra.Command{
	Use:   "comment-issue",
	Short: "Comment Issue",
	Long:  "Comment Issue",
	Run: func(cmd *cobra.Command, args []string) {
		token, _ := cmd.Flags().GetString("token")
		repo, _ := cmd.Flags().GetString("repo")
		server, _ := cmd.Flags().GetString("server")
		date, _ := cmd.Flags().GetString("date")
		netdataPort, _ := cmd.Flags().GetString("netdata-port")

		err := commentIssue(token, repo, server, date, netdataPort)
		if err != nil {
			fmt.Println("error: ", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(commentIssueCmd)

	commentIssueCmd.Flags().StringP("token", "t", "", "GitHub API token")
	commentIssueCmd.Flags().StringP("repo", "r", "", "GitHub repository (Saza-ku/isucon13q)")
	commentIssueCmd.Flags().StringP("server", "s", "", "Server (isucon1)")
	commentIssueCmd.Flags().StringP("date", "d", "", "Date (10121200)")
	commentIssueCmd.Flags().StringP("netdata-port", "p", "", "Netdata port (19991)")
}

func commentIssue(token string, repo string, server string, date string, netdataPort string) error {
	client := getClient(token)

	repo, name, err := parseRepo(repo)
	if err != nil {
		return err
	}

	issue, err := getLatestIssue(client, repo, name)
	if err != nil {
		return err
	}

	comment, err := buildComment(server, date, netdataPort)
	if err != nil {
		return err
	}

	_, _, err = client.Issues.CreateComment(context.Background(), repo, name, *issue.Number, &github.IssueComment{
		Body: github.String(comment),
	})
	if err != nil {
		return err
	}

	return nil
}

type Comment struct {
	Server       string
	Alp          string
	MySQLSlowLog string
	MySQLExplain string
	Netdata      string
	NetdataPort  string
	Date         string
	Jaeger       string
}

func buildComment(server string, date string, netdataPort string) (string, error) {
	tmpl, err := template.New("comment").Parse(`
## {{.Server}}
### alp
` + "```" + `
{{.Alp}}
` + "```" + `

### slow query
` + "```" + `
{{.MySQLSlowLog}}
` + "```" + `

### explain
<details>
<summary>open</summary>

` + "```" + `
{{.MySQLExplain}}
` + "```" + `

</details>

### netdata
{{.Netdata}}

` + "```" + `
ssh -fNT -L 0.0.0.0:{{.NetdataPort}}:127.0.0.1:19999 isucon@{{.Server}}
` + "```" + `

### jaeger
{{.Jaeger}}

### pprof
` + "```" + `
go tool pprof -http=0.0.0.0:1080 http://{{.Server}}/pprof/{{.Date}}
` + "```" + `

### fgprof
` + "```" + `
go tool pprof -http=0.0.0.0:1080 http://{{.Server}}/pprof/fg-{{.Date}}
` + "```" + `
`)
	if err != nil {
		return "", err
	}

	alp, err := readFile(fmt.Sprintf("/home/isucon/results/%s/alp.log", date))
	if err != nil {
		return "", err
	}

	slowQuery, err := readFile(fmt.Sprintf("/home/isucon/results/%s/mysql-slow.log", date))
	if err != nil {
		return "", err
	}

	explain, err := readFile(fmt.Sprintf("/home/isucon/results/%s/mysql-explain.log", date))
	if err != nil {
		return "", err
	}

	netdata, err := readFile(fmt.Sprintf("/home/isucon/results/%s/netdata.txt", date))
	if err != nil {
		return "", err
	}

	jaeger, err := readFile(fmt.Sprintf("/home/isucon/results/%s/jaeger.txt", date))
	if err != nil {
		return "", err
	}

	comment := Comment{
		Server:       server,
		Alp:          alp,
		MySQLSlowLog: slowQuery,
		MySQLExplain: explain,
		Netdata:      netdata,
		NetdataPort:  netdataPort,
		Date:         date,
		Jaeger:       jaeger,
	}

	var b bytes.Buffer
	err = tmpl.Execute(&b, comment)
	if err != nil {
		return "", err
	}

	return b.String(), nil
}

func getLatestIssue(client *github.Client, repo string, name string) (*github.Issue, error) {
	issues, _, err := client.Issues.ListByRepo(context.Background(), repo, name, &github.IssueListByRepoOptions{
		State: "open",
	})
	if err != nil {
		return nil, err
	}

	measureIssues := make([]*github.Issue, 0)
	for _, issue := range issues {
		for _, label := range issue.Labels {
			if *label.Name == "measure" {
				measureIssues = append(measureIssues, issue)
				break
			}
		}
	}

	lastNumber := 0
	var latestIssue *github.Issue = nil
	for _, issue := range measureIssues {
		if *issue.Number > lastNumber {
			lastNumber = *issue.Number
			latestIssue = issue
		}
	}

	if latestIssue == nil {
		return nil, fmt.Errorf("issue not found")
	}

	return latestIssue, nil
}

func readFile(path string) (string, error) {
	r, err := os.Open(path)
	if err != nil {
		return "", err
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
