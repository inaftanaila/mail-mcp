package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/dastrobu/mail-mcp/internal/jxa"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed scripts/archive_messages.js
var archiveMessagesScript string

// ArchiveMessagesInput defines input parameters for the archive_messages tool.
type ArchiveMessagesInput struct {
	Account     string   `json:"account" jsonschema:"Name of the email account the messages are in" long:"account" description:"Name of the email account the messages are in"`
	MailboxPath []string `json:"mailbox_path" jsonschema:"Path to the source mailbox as an array (e.g. ['Inbox'] for top-level or ['Inbox','Subfolder'] for nested). Note: Mailbox names are case-sensitive." long:"mailbox-path" description:"Path to the source mailbox. Can be specified multiple times for nested paths."`
	MessageIDs  []string `json:"message_ids" jsonschema:"RFC822 Message-IDs of the messages to archive (the value returned in get_message_content's messageId field, NOT the numeric id). Angle brackets are optional." long:"message-id" description:"An RFC822 Message-ID to archive. Specify multiple times for a batch."`
}

// RegisterArchiveMessages registers the archive_messages (batch) tool with the MCP server.
func RegisterArchiveMessages(srv *mcp.Server) {
	mcp.AddTool(srv,
		&mcp.Tool{
			Name:        "archive_messages",
			Description: "Batch-archives messages by moving them to the account's top-level \"Archive\" mailbox, located by their RFC822 Message-ID (the messageId field from get_message_content, NOT the numeric id). Enumerates the source mailbox once and moves every match in a single pass - use this for bulk inbox clears instead of calling archive_message in a loop. Resolves the Archive target automatically; there is no archive-target parameter. Works for Exchange/iCloud/IMAP accounts. NOTE: Apple Mail cannot archive Gmail accounts; for Gmail this tool hard-errors with GMAIL_ARCHIVE_UNSUPPORTED and moves nothing (use the standalone gmail_archive.py IMAP tool instead). If no Archive mailbox exists, the whole call errors rather than falling back to Trash or Inbox. Each requested Message-ID is reported as archived, not_found, or error.",
			InputSchema: GenerateSchema[ArchiveMessagesInput](),
			Annotations: &mcp.ToolAnnotations{
				Title:           "Archive Messages (batch)",
				ReadOnlyHint:    false,
				IdempotentHint:  true,
				DestructiveHint: new(true),
				OpenWorldHint:   new(true),
			},
		},
		HandleArchiveMessages,
	)
}

func HandleArchiveMessages(ctx context.Context, request *mcp.CallToolRequest, input ArchiveMessagesInput) (*mcp.CallToolResult, any, error) {
	// Validate inputs
	if input.Account == "" {
		return nil, nil, fmt.Errorf("account is required")
	}
	if len(input.MailboxPath) == 0 {
		return nil, nil, fmt.Errorf("mailbox_path is required and must be a non-empty array")
	}
	if len(input.MessageIDs) == 0 {
		return nil, nil, fmt.Errorf("message_ids is required and must be a non-empty array")
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
	data, err := jxa.Execute(ctx, archiveMessagesScript, string(inputJSON))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute archive_messages: %w", err)
	}

	return nil, data, nil
}
