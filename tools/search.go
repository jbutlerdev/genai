package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"golang.org/x/net/html"
)

var searchTools = map[string]Tool{
	"SearchWeb":    searchWebTool,
	"RetrievePage": retrievePageTool,
}

var searchWebTool = Tool{
	Name:        "SearchWeb",
	Description: "Search the web for information",
	Parameters: []Parameter{
		{
			Name:        "query",
			Type:        "string",
			Description: "The query to search the web for",
		},
	},
	Options: map[string]string{},
	Run:     SearchWeb,
}

func SearchWeb(args map[string]any) (map[string]any, error) {
	query, ok := args["query"].(string)
	if !ok {
		return map[string]any{
			"success": false,
			"error":   "query is not a string",
		}, fmt.Errorf("query is not a string")
	}

	// get searxngURL from environment variable
	searxngURL := os.Getenv("SEARXNG_URL")
	if searxngURL == "" {
		return map[string]any{
			"success": false,
			"error":   "SEARXNG_URL is not set",
		}, fmt.Errorf("SEARXNG_URL is not set")
	}

	encodedQuery := url.QueryEscape(query)
	url := fmt.Sprintf("%s/?q=%s&format=json", searxngURL, encodedQuery)

	resp, err := http.Get(url)
	if err != nil {
		return map[string]any{
			"success": false,
			"error":   err.Error(),
		}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return map[string]any{
			"success": false,
			"error":   err.Error(),
		}, err
	}

	// parse body to json indented string
	var jsonBody map[string]any
	if err := json.Unmarshal(body, &jsonBody); err != nil {
		return map[string]any{
			"success": false,
			"error":   err.Error(),
		}, err
	}

	return map[string]any{
		"success": true,
		"results": jsonBody,
	}, nil
}

var retrievePageTool = Tool{
	Name:        "RetrievePage",
	Description: "Retrieve the web page contents from a URL",
	Parameters: []Parameter{
		{
			Name:        "url",
			Type:        "string",
			Description: "The URL to retrieve the web page contents from",
		},
	},
	Options:   map[string]string{},
	Run:       RetrievePage,
	Summarize: true,
}

func RetrievePage(args map[string]any) (map[string]any, error) {
	urlStr, ok := args["url"].(string)
	if !ok {
		return map[string]any{
			"success": false,
			"error":   "url is not a string",
		}, fmt.Errorf("url is not a string")
	}

	// Parse URL to check domain
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return map[string]any{
			"success": false,
			"error":   "invalid URL",
		}, fmt.Errorf("invalid URL: %v", err)
	}

	// Check if it's a YouTube domain
	if parsedURL.Host == "youtube.com" || parsedURL.Host == "www.youtube.com" || 
	   parsedURL.Host == "youtu.be" || parsedURL.Host == "m.youtube.com" {
		return map[string]any{
			"success": false,
			"error":   "cannot retrieve video content",
		}, fmt.Errorf("error cannot retrieve video content")
	}

	resp, err := http.Get(urlStr)
	if err != nil {
		return map[string]any{
			"success": false,
			"error":   err.Error(),
		}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return map[string]any{
			"success": false,
			"error":   fmt.Sprintf("status code: %d", resp.StatusCode),
		}, fmt.Errorf("status code: %d", resp.StatusCode)
	}

	bodyText, err := extractBody(resp.Body)
	if err != nil {
		return map[string]any{
			"success": false,
			"error":   err.Error(),
		}, err
	}
	return map[string]any{
		"success": true,
		"body":    bodyText,
	}, nil
}

func extractBody(r io.Reader) (string, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return "", err
	}

	var bodyText string
	var traverse func(*html.Node)

	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "body" {
			bodyText = extractText(n)
			return // Stop traversing after finding the body
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}

	traverse(doc)
	return bodyText, nil
}

func extractText(n *html.Node) string {
	var text string
	if n.Type == html.TextNode {
		text = n.Data
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		text += extractText(c)
	}
	return text
}
