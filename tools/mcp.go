package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func StartMCPServer() {
	// Create a new MCP server
	s := server.NewMCPServer(
		"GenAI Tools",
		"0.0.1",
		server.WithResourceCapabilities(true, true),
		server.WithLogging(),
	)

    for name, tool := range toolMap {
        mcpTool := mcp.NewTool(name,
            mcp.WithDescription(tool.Description),
        )

        for _, param := range tool.Parameters {
            if param.Type == "string" {
                mcpTool.AddParameter(mcp.WithString(param.Name,
                    mcp.Description(param.Description),
                    mcp.RequiredIf(param.Required),
                ))
            } else if param.Type == "number" {
                 mcpTool.AddParameter(mcp.WithNumber(param.Name,
                    mcp.Description(param.Description),
                     mcp.RequiredIf(param.Required),
                ))
            } else if param.Type == "boolean" {
                mcpTool.AddParameter(mcp.WithBool(param.Name,
                    mcp.Description(param.Description),
                    mcp.RequiredIf(param.Required),
                ))
            }
        }

        s.AddTool(mcpTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
            params := make(map[string]any)
            for key, value := range request.Params.Arguments {
                params[key] = value
            }

            result, err := tool.Run(params)
            if err != nil {
                return mcp.NewToolResultError(err.Error()), nil
            }

            if text, ok := result["text"].(string); ok {
                return mcp.NewToolResultText(text), nil
            } else {
                return mcp.NewToolResultError("Unexpected result format"), nil
            }

        })
    }



	// Start the server
	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
