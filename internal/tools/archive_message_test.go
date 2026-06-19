package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestHandleArchiveMessage_InputValidation(t *testing.T) {
	tests := []struct {
		name        string
		input       ArchiveMessageInput
		wantErrPart string
	}{
		{
			name: "missing account",
			input: ArchiveMessageInput{
				MailboxPath: []string{"Inbox"},
				MessageID:   123,
			},
			wantErrPart: "account is required",
		},
		{
			name: "missing mailbox_path",
			input: ArchiveMessageInput{
				Account:   "PRK",
				MessageID: 123,
			},
			wantErrPart: "mailbox_path is required",
		},
		{
			name: "empty mailbox_path",
			input: ArchiveMessageInput{
				Account:     "PRK",
				MailboxPath: []string{},
				MessageID:   123,
			},
			wantErrPart: "mailbox_path is required",
		},
		{
			name: "missing message_id",
			input: ArchiveMessageInput{
				Account:     "PRK",
				MailboxPath: []string{"Inbox"},
			},
			wantErrPart: "message_id is required",
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := HandleArchiveMessage(ctx, &mcp.CallToolRequest{}, tt.input)
			if err == nil {
				t.Fatalf("expected validation error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErrPart) {
				t.Errorf("expected error to contain %q, got: %s", tt.wantErrPart, err.Error())
			}
		})
	}
}
