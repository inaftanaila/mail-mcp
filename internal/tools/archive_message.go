package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/dastrobu/mail-mcp/internal/jxa"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed scripts/archive_message.js
var archiveMessageScript string

// ArchiveMessageInput defines input parameters for the archive_message tool.
type ArchiveMessageInput struct {
	Account     string   `json:"account" jsonschema:"Name of the email account the message is in" long:"account" description:"Name of the email account the message is in"`
	MailboxPath []string `json:"mailbox_path" jsonschema:"Path to the source mailbox as an array (e.g. ['Inbox'] for top-level or ['Inbox','Subfolder'] for nested). Note: Mailbox names are case-sensitive." long:"mailbox-path" description:"Path to the source mailbox. Can be specified multiple times for nested paths."`
	MessageID   int      `json:"message_id" jsonschema:"The unique Mail.app ID of the message to archive" long:"message-id" description:"The unique Mail.app ID of the message to archive"`
}

// RegisterArchiveMessage registers the archive_message tool with the MCP server.
func RegisterArchiveMessage(srv *mcp.Server) {
	mcp.AddTool(srv,
		&mcp.Tool{
			Name:        "archive_message",
			Description: "Archives a message by moving it to the account's top-level \"Archive\" mailbox. Resolves the Archive target automatically; there is no archive-target parameter. Works for Exchange/iCloud/IMAP accounts. NOTE: Apple Mail cannot archive Gmail accounts; for Gmail this tool hard-errors with GMAIL_ARCHIVE_UNSUPPORTED and does not move the message (use the standalone gmail_archive.py IMAP tool instead). If the message is already in Archive, this is a noop. If no Archive mailbox exists, it errors rather than falling back to Trash or Inbox.",
			InputSchema: GenerateSchema[ArchiveMessageInput](),
			Annotations: &mcp.ToolAnnotations{
				Title:           "Archive Message",
				ReadOnlyHint:    false,
				IdempotentHint:  true,
				DestructiveHint: new(true),
				OpenWorldHint:   new(true),
			},
		},
		HandleArchiveMessage,
	)
}

func HandleArchiveMessage(ctx context.Context, request *mcp.CallToolRequest, input ArchiveMessageInput) (*mcp.CallToolResult, any, error) {
	// Validate inputs
	if input.Account == "" {
		return nil, nil, fmt.Errorf("account is required")
	}
	if len(input.MailboxPath) == 0 {
		return nil, nil, fmt.Errorf("mailbox_path is required and must be a non-empty array")
	}
	if input.MessageID == 0 {
		return nil, nil, fmt.Errorf("message_id is required and must be a positive integer")
	}

	// Enforce account access policy BEFORE running any JXA.
	if err := enforceAccountAccess(input.Account); err != nil {
		return nil, nil, err
	}

	// Marshal input to JSON
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal input for JXA: %w", err)
	}

	// Execute JXA script with input as JSON string
	data, err := jxa.Execute(ctx, archiveMessageScript, string(inputJSON))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute archive_message: %w", err)
	}

	return nil, data, nil
}
