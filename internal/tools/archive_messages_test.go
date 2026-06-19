package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestHandleArchiveMessages_InputValidation(t *testing.T) {
	tests := []struct {
		name        string
		input       ArchiveMessagesInput
		wantErrPart string
	}{
		{
			name: "missing account",
			input: ArchiveMessagesInput{
				MailboxPath: []string{"Inbox"},
				MessageIDs:  []string{"<abc@example.com>"},
			},
			wantErrPart: "account is required",
		},
		{
			name: "missing mailbox_path",
			input: ArchiveMessagesInput{
				Account:    "PRK",
				MessageIDs: []string{"<abc@example.com>"},
			},
			wantErrPart: "mailbox_path is required",
		},
		{
			name: "empty mailbox_path",
			input: ArchiveMessagesInput{
				Account:     "PRK",
				MailboxPath: []string{},
				MessageIDs:  []string{"<abc@example.com>"},
			},
			wantErrPart: "mailbox_path is required",
		},
		{
			name: "missing message_ids",
			input: ArchiveMessagesInput{
				Account:     "PRK",
				MailboxPath: []string{"Inbox"},
			},
			wantErrPart: "message_ids is required",
		},
		{
			name: "empty message_ids",
			input: ArchiveMessagesInput{
				Account:     "PRK",
				MailboxPath: []string{"Inbox"},
				MessageIDs:  []string{},
			},
			wantErrPart: "message_ids is required",
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := HandleArchiveMessages(ctx, &mcp.CallToolRequest{}, tt.input)
			if err == nil {
				t.Fatalf("expected validation error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErrPart) {
				t.Errorf("expected error to contain %q, got: %s", tt.wantErrPart, err.Error())
			}
		})
	}
}
