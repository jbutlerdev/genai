package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/google/go-github/v60/github"
	"golang.org/x/oauth2"
)

const (
	// Environment variable name for GitHub token
	GithubTokenEnv = "GITHUB_TOKEN"
)

var githubTools = map[string]Tool{
	"getPullRequests":     getPullRequestsTool,
	"getAssignedPRs":      getAssignedPRsTool,
	"getUserRepos":        getUserReposTool,
	"getContributedRepos": getContributedReposTool,
	"getAssignedIssues":   getAssignedIssuesTool,
	"getInvolvedIssues":   getInvolvedIssuesTool,
}

// getGitHubToken gets the GitHub token from environment variable
func getGitHubToken() (string, error) {
	token := os.Getenv(GithubTokenEnv)
	if token == "" {
		return "", fmt.Errorf("GitHub token not found in environment variable %s", GithubTokenEnv)
	}
	return token, nil
}

// getGitHubClient creates a new GitHub client using the token from environment
func getGitHubClient() (*github.Client, error) {
	token, err := getGitHubToken()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	return github.NewClient(tc), nil
}

var getPullRequestsTool = Tool{
	Name:        "getPullRequests",
	Description: "Get pull requests a user is active in",
	Parameters: []Parameter{
		{
			Name:        "user",
			Type:        "string",
			Description: "GitHub username",
			Required:    true,
		},
		{
			Name:        "repository",
			Type:        "string",
			Description: "Repository name in owner/repo format (optional)",
			Required:    false,
		},
	},
	Run: GetPullRequests,
}

func GetPullRequests(args map[string]any) (map[string]any, error) {
	user := args["user"].(string)
	repo, hasRepo := args["repository"].(string)

	client, err := getGitHubClient()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	query := fmt.Sprintf("involves:%s is:pr", user)
	if hasRepo {
		query += fmt.Sprintf(" repo:%s", repo)
	}

	result, _, err := client.Search.Issues(ctx, query, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to search pull requests: %w", err)
	}

	prs := make([]map[string]string, len(result.Issues))
	for i, pr := range result.Issues {
		prs[i] = map[string]string{
			"number":    strconv.Itoa(pr.GetNumber()),
			"title":     pr.GetTitle(),
			"state":     pr.GetState(),
			"url":       pr.GetHTMLURL(),
			"repo":      strings.TrimPrefix(pr.GetRepositoryURL(), "https://api.github.com/repos/"),
			"createdAt": pr.GetCreatedAt().String(),
			"updatedAt": pr.GetUpdatedAt().String(),
		}
	}

	marshaled, err := json.Marshal(prs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal pull requests: %w", err)
	}

	if DEBUG {
		fmt.Printf("called getPullRequests with %s\nFound %d pull requests\nInfo: %s\n", user, result.GetTotal(), string(marshaled))
	}

	return map[string]any{
		"pullRequests": string(marshaled),
		"total":        result.GetTotal(),
	}, nil
}

var getAssignedPRsTool = Tool{
	Name:        "getAssignedPRs",
	Description: "Get pull requests assigned to a user",
	Parameters: []Parameter{
		{
			Name:        "user",
			Type:        "string",
			Description: "GitHub username",
			Required:    true,
		},
		{
			Name:        "repository",
			Type:        "string",
			Description: "Repository name in owner/repo format (optional)",
			Required:    false,
		},
	},
	Run: GetAssignedPRs,
}

func GetAssignedPRs(args map[string]any) (map[string]any, error) {
	user := args["user"].(string)
	repo, hasRepo := args["repository"].(string)

	client, err := getGitHubClient()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	query := fmt.Sprintf("assignee:%s is:pr", user)
	if hasRepo {
		query += fmt.Sprintf(" repo:%s", repo)
	}

	result, _, err := client.Search.Issues(ctx, query, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to search assigned pull requests: %w", err)
	}

	prs := make([]map[string]string, len(result.Issues))
	for i, pr := range result.Issues {
		prs[i] = map[string]string{
			"number":    strconv.Itoa(pr.GetNumber()),
			"title":     pr.GetTitle(),
			"state":     pr.GetState(),
			"url":       pr.GetHTMLURL(),
			"repo":      strings.TrimPrefix(pr.GetRepositoryURL(), "https://api.github.com/repos/"),
			"createdAt": pr.GetCreatedAt().String(),
			"updatedAt": pr.GetUpdatedAt().String(),
		}
	}

	marshaled, err := json.Marshal(prs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal pull requests: %w", err)
	}

	if DEBUG {
		fmt.Printf("called getAssignedPRs with %s\nFound %d pull requests\nInfo: %s\n", user, result.GetTotal(), string(marshaled))
	}

	return map[string]any{
		"pullRequests": string(marshaled),
		"total":        result.GetTotal(),
	}, nil
}

var getUserReposTool = Tool{
	Name:        "getUserRepos",
	Description: "Get repositories owned by a user",
	Parameters: []Parameter{
		{
			Name:        "user",
			Type:        "string",
			Description: "GitHub username",
			Required:    true,
		},
	},
	Run: GetUserRepos,
}

func GetUserRepos(args map[string]any) (map[string]any, error) {
	user := args["user"].(string)

	client, err := getGitHubClient()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	opts := &github.RepositoryListByUserOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	repos, _, err := client.Repositories.ListByUser(ctx, user, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list user repositories: %w", err)
	}

	repoList := make([]map[string]interface{}, 0)
	for _, repo := range repos {
		repoList = append(repoList, map[string]interface{}{
			"name":        repo.GetName(),
			"fullName":    repo.GetFullName(),
			"description": repo.GetDescription(),
			"url":         repo.GetHTMLURL(),
			"language":    repo.GetLanguage(),
			"stars":       repo.GetStargazersCount(),
			"forks":       repo.GetForksCount(),
			"createdAt":   repo.GetCreatedAt(),
			"updatedAt":   repo.GetUpdatedAt(),
		})
	}
	marshaled, err := json.Marshal(repoList)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal repository list: %w", err)
	}
	if DEBUG {
		fmt.Printf("called getUserRepos with %s\nFound %d repositories\nInfo: %s\n", user, len(repoList), string(marshaled))
	}

	return map[string]any{
		"repositories": string(marshaled),
		"total":        len(repoList),
	}, nil
}

var getContributedReposTool = Tool{
	Name:        "getContributedRepos",
	Description: "Get repositories a user has contributed to",
	Parameters: []Parameter{
		{
			Name:        "user",
			Type:        "string",
			Description: "GitHub username",
			Required:    true,
		},
	},
	Run: GetContributedRepos,
}

func GetContributedRepos(args map[string]any) (map[string]any, error) {
	user := args["user"].(string)

	client, err := getGitHubClient()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	query := fmt.Sprintf("author:%s", user)
	result, _, err := client.Search.Repositories(ctx, query, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to search contributed repositories: %w", err)
	}

	repoList := make([]map[string]string, len(result.Repositories))
	for i, repo := range result.Repositories {
		repoList[i] = map[string]string{
			"name":        repo.GetName(),
			"fullName":    repo.GetFullName(),
			"description": repo.GetDescription(),
			"url":         repo.GetHTMLURL(),
			"language":    repo.GetLanguage(),
			"stars":       strconv.Itoa(repo.GetStargazersCount()),
			"forks":       strconv.Itoa(repo.GetForksCount()),
			"createdAt":   repo.GetCreatedAt().String(),
			"updatedAt":   repo.GetUpdatedAt().String(),
		}
	}

	marshaled, err := json.Marshal(repoList)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal repository list: %w", err)
	}
	if DEBUG {
		fmt.Printf("called getContributedRepos with %s\nFound %d repositories\nInfo: %s\n", user, result.GetTotal(), string(marshaled))
	}

	return map[string]any{
		"repositories": string(marshaled),
		"total":        result.GetTotal(),
	}, nil
}

var getAssignedIssuesTool = Tool{
	Name:        "getAssignedIssues",
	Description: "Get issues assigned to a user",
	Parameters: []Parameter{
		{
			Name:        "user",
			Type:        "string",
			Description: "GitHub username",
			Required:    true,
		},
		{
			Name:        "repository",
			Type:        "string",
			Description: "Repository name in owner/repo format (optional)",
			Required:    false,
		},
	},
	Run: GetAssignedIssues,
}

func GetAssignedIssues(args map[string]any) (map[string]any, error) {
	user := args["user"].(string)
	repo, hasRepo := args["repository"].(string)

	client, err := getGitHubClient()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	query := fmt.Sprintf("assignee:%s is:issue", user)
	if hasRepo {
		query += fmt.Sprintf(" repo:%s", repo)
	}

	result, _, err := client.Search.Issues(ctx, query, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to search assigned issues: %w", err)
	}

	issues := make([]map[string]string, len(result.Issues))
	for i, issue := range result.Issues {
		issues[i] = map[string]string{
			"number":    strconv.Itoa(issue.GetNumber()),
			"title":     issue.GetTitle(),
			"state":     issue.GetState(),
			"url":       issue.GetHTMLURL(),
			"repo":      strings.TrimPrefix(issue.GetRepositoryURL(), "https://api.github.com/repos/"),
			"createdAt": issue.GetCreatedAt().String(),
			"updatedAt": issue.GetUpdatedAt().String(),
		}
	}

	marshaled, err := json.Marshal(issues)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal issues: %w", err)
	}

	if DEBUG {
		fmt.Printf("called getAssignedIssues with %s\nFound %d issues\nInfo: %s\n", user, result.GetTotal(), string(marshaled))
	}

	return map[string]any{
		"issues": string(marshaled),
		"total":  result.GetTotal(),
	}, nil
}

var getInvolvedIssuesTool = Tool{
	Name:        "getInvolvedIssues",
	Description: "Get issues a user has been involved in",
	Parameters: []Parameter{
		{
			Name:        "user",
			Type:        "string",
			Description: "GitHub username",
			Required:    true,
		},
		{
			Name:        "repository",
			Type:        "string",
			Description: "Repository name in owner/repo format (optional)",
			Required:    false,
		},
	},
	Run: GetInvolvedIssues,
}

func GetInvolvedIssues(args map[string]any) (map[string]any, error) {
	user := args["user"].(string)
	repo, hasRepo := args["repository"].(string)

	client, err := getGitHubClient()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	query := fmt.Sprintf("involves:%s is:issue", user)
	if hasRepo {
		query += fmt.Sprintf(" repo:%s", repo)
	}

	result, _, err := client.Search.Issues(ctx, query, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to search involved issues: %w", err)
	}

	issues := make([]map[string]string, len(result.Issues))
	for i, issue := range result.Issues {
		issues[i] = map[string]string{
			"number":    strconv.Itoa(issue.GetNumber()),
			"title":     issue.GetTitle(),
			"state":     issue.GetState(),
			"url":       issue.GetHTMLURL(),
			"repo":      strings.TrimPrefix(issue.GetRepositoryURL(), "https://api.github.com/repos/"),
			"createdAt": issue.GetCreatedAt().String(),
			"updatedAt": issue.GetUpdatedAt().String(),
		}
	}

	marshaled, err := json.Marshal(issues)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal issues: %w", err)
	}

	if DEBUG {
		fmt.Printf("called getInvolvedIssues with %s\nFound %d issues\nInfo: %s\n", user, result.GetTotal(), string(marshaled))
	}

	return map[string]any{
		"issues": string(marshaled),
		"total":  result.GetTotal(),
	}, nil
}
