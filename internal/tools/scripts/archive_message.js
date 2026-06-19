#!/usr/bin/osascript -l JavaScript

/**
 * Archive a message in Mail.app by moving it to the account's Archive mailbox.
 *
 * Arguments:
 *   argv[0] - JSON string containing:
 *     - account (required)
 *     - mailbox_path (required) - Source mailbox path array, e.g. ["Inbox"]
 *     - message_id (required) - numeric Mail.app message id
 *
 * Behaviour:
 *   - Exchange/iCloud/IMAP (non-Gmail): moves the message to the top-level
 *     "Archive" mailbox on the account. If the message is already in "Archive",
 *     it is a noop. If no "Archive" mailbox exists, it hard-errors (never falls
 *     back to Trash or Inbox).
 *   - Gmail: Apple Mail cannot relocate Gmail messages (move/delete are no-ops),
 *     so this hard-errors with GMAIL_ARCHIVE_UNSUPPORTED and does NOT touch the
 *     message. Use the standalone gmail_archive.py IMAP tool instead.
 *
 * Notes:
 *   - acct.mailboxes.byName("...") throws -1728 on names with spaces, so
 *     mailboxes are resolved by ITERATING acct.mailboxes() and matching .name().
 *   - accountType is unreliable (Exchange and Gmail both report "imap"), so
 *     Gmail is detected via serverName() host match with an All-Mail+Bin shape
 *     fallback.
 */

function run(argv) {
  const Mail = Application("Mail");
  Mail.includeStandardAdditions = true;

  // Check if Mail.app is running
  if (!Mail.running()) {
    return JSON.stringify({
      success: false,
      error: "Mail.app is not running. Please start Mail.app and try again.",
      errorCode: "MAIL_APP_NOT_RUNNING",
    });
  }

  // Parse arguments
  let args;
  try {
    args = JSON.parse(argv[0]);
  } catch (e) {
    return JSON.stringify({
      success: false,
      error: "Failed to parse input arguments JSON",
    });
  }

  const accountName = args.account || "";
  const mailboxPath = args.mailbox_path || [];
  const messageId = args.message_id ? parseInt(args.message_id) : 0;

  // Validate all required arguments explicitly
  if (!accountName) {
    return JSON.stringify({
      success: false,
      error: "Account name is required",
    });
  }

  if (!Array.isArray(mailboxPath) || mailboxPath.length === 0) {
    return JSON.stringify({
      success: false,
      error: "Mailbox path is required and must be a non-empty array",
    });
  }

  if (!messageId || messageId < 1) {
    return JSON.stringify({
      success: false,
      error: "Message ID is required and must be a positive integer",
    });
  }

  // Robust mailbox traversal function (reused verbatim from get_message_content.js)
  function findMailboxByPath(account, targetPath) {
    if (!targetPath || targetPath.length === 0) return account;

    try {
      let current = account;
      for (let i = 0; i < targetPath.length; i++) {
        const part = targetPath[i];
        let next = null;
        try {
          next = current.mailboxes.whose({ name: part })()[0];
        } catch (e) {}

        if (!next) {
          try {
            next = current.mailboxes[part];
            next.name();
          } catch (e) {}
        }
        if (!next) throw new Error("not found");
        current = next;
      }
      return current;
    } catch (e) {}

    try {
      const allMailboxes = account.mailboxes();
      for (let i = 0; i < allMailboxes.length; i++) {
        const mbx = allMailboxes[i];
        const path = [];
        let current = mbx;
        while (current) {
          try {
            const name = current.name();
            if (name === account.name()) break;
            path.unshift(name);
            current = current.container();
          } catch (e) {
            break;
          }
        }
        if (path.length === targetPath.length) {
          let match = true;
          for (let j = 0; j < path.length; j++) {
            if (path[j] !== targetPath[j]) {
              match = false;
              break;
            }
          }
          if (match) return mbx;
        }
      }
    } catch (e) {}
    return null;
  }

  // Detect a Gmail account. accountType is unreliable (both Exchange and Gmail
  // report "imap"), so prefer the server host, then fall back to a shape-check
  // (Gmail exposes both an All Mail and a Bin/Trash mailbox).
  function isGmailAccount(account) {
    try {
      const server = (account.serverName() || "").toLowerCase();
      if (server.indexOf("gmail.com") !== -1 || server.indexOf("googlemail.com") !== -1) {
        return true;
      }
    } catch (e) {}

    try {
      const allMailboxes = account.mailboxes();
      let hasAllMail = false;
      let hasBin = false;
      for (let i = 0; i < allMailboxes.length; i++) {
        let name = "";
        try {
          name = (allMailboxes[i].name() || "").toLowerCase();
        } catch (e) {
          continue;
        }
        if (name === "all mail") hasAllMail = true;
        if (name === "bin" || name === "trash") hasBin = true;
      }
      if (hasAllMail && hasBin) return true;
    } catch (e) {}

    return false;
  }

  // Resolve the destination Archive mailbox by iterating the account's
  // mailboxes (byName throws -1728 on spaced names, and we want an exact match).
  function findArchiveMailbox(account) {
    try {
      const allMailboxes = account.mailboxes();
      for (let i = 0; i < allMailboxes.length; i++) {
        let name = "";
        try {
          name = allMailboxes[i].name();
        } catch (e) {
          continue;
        }
        if (name === "Archive") return allMailboxes[i];
      }
    } catch (e) {}
    return null;
  }

  try {
    // Find the account
    let targetAccount;
    try {
      targetAccount = Mail.accounts[accountName];
      targetAccount.name();
    } catch (e) {
      return JSON.stringify({
        success: false,
        error: `Account "${accountName}" not found. Please verify the account name is correct.`,
        errorCode: "ACCOUNT_NOT_FOUND",
      });
    }

    // Locate the source mailbox
    const targetMailbox = findMailboxByPath(targetAccount, mailboxPath);
    if (!targetMailbox) {
      return JSON.stringify({
        success: false,
        error:
          "Mailbox path '" +
          mailboxPath.join(" > ") +
          "' not found in account '" +
          accountName +
          "'.",
        errorCode: "MAILBOX_NOT_FOUND",
      });
    }

    // Locate the message by id within the source mailbox
    const matchingMessages = targetMailbox.messages.whose({ id: messageId })();
    if (!matchingMessages || matchingMessages.length === 0) {
      return JSON.stringify({
        success: false,
        error: `Message with ID ${messageId} not found in mailbox "${mailboxPath.join(" > ")}". The message may have been deleted or moved.`,
        errorCode: "MESSAGE_NOT_FOUND",
      });
    }
    const targetMessage = matchingMessages[0];

    // Gmail cannot be archived from Apple Mail -> hard error, do NOT move/delete.
    if (isGmailAccount(targetAccount)) {
      return JSON.stringify({
        success: false,
        error:
          "Apple Mail cannot archive Gmail; use the gmail_archive.py IMAP tool.",
        errorCode: "GMAIL_ARCHIVE_UNSUPPORTED",
      });
    }

    // Resolve the Archive mailbox; never fall back to Trash/Inbox.
    const archive = findArchiveMailbox(targetAccount);
    if (!archive) {
      return JSON.stringify({
        success: false,
        error: `No "Archive" mailbox found on account "${accountName}". Cannot archive this message.`,
        errorCode: "ARCHIVE_MAILBOX_NOT_FOUND",
      });
    }

    // If the message is already in Archive, this is a noop.
    let currentMailboxName = "";
    try {
      currentMailboxName = targetMessage.mailbox().name();
    } catch (e) {}
    if (currentMailboxName === "Archive") {
      return JSON.stringify({
        success: true,
        data: {
          strategy: "noop-already-archived",
          resolvedArchive: "Archive",
          account: accountName,
          from: mailboxPath,
          message_id: messageId,
          moved: false,
        },
      });
    }

    // Move the message to Archive.
    Mail.move(targetMessage, { to: archive });

    return JSON.stringify({
      success: true,
      data: {
        strategy: "move-to-archive",
        resolvedArchive: "Archive",
        account: accountName,
        from: mailboxPath,
        message_id: messageId,
        moved: true,
      },
    });
  } catch (e) {
    return JSON.stringify({
      success: false,
      error: `Failed to archive message: ${e.toString()}`,
      errorCode: "ARCHIVE_FAILED",
    });
  }
}
