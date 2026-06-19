#!/usr/bin/osascript -l JavaScript

/**
 * Batch-archive messages in Mail.app, keyed by their RFC822 Message-ID.
 *
 * This is the bulk counterpart to archive_message.js. It enumerates the source
 * mailbox ONCE and moves every requested message in a single Mail.app pass,
 * which matters when clearing a large inbox (hundreds of messages).
 *
 * Arguments:
 *   argv[0] - JSON string containing:
 *     - account (required)
 *     - mailbox_path (required) - Source mailbox path array, e.g. ["Inbox"]
 *     - message_ids (required) - array of RFC822 Message-IDs (the value Mail.app
 *       exposes as a message's `messageId` property, NOT the numeric `id`).
 *       Angle brackets are optional; matching is tolerant of surrounding <>.
 *
 * Behaviour:
 *   - Exchange/iCloud/IMAP (non-Gmail): moves each found message to the
 *     top-level "Archive" mailbox on the account. If no "Archive" mailbox
 *     exists, the whole call hard-errors (never falls back to Trash or Inbox).
 *   - Gmail: Apple Mail cannot relocate Gmail messages, so the whole call
 *     hard-errors with GMAIL_ARCHIVE_UNSUPPORTED and touches nothing. Use the
 *     standalone gmail_archive.py IMAP tool instead.
 *
 * Per-message outcome (only when the account is archivable and Archive exists):
 *   - "archived"   - found in the source mailbox and moved.
 *   - "not_found"  - no message with that Message-ID in the source mailbox.
 *   - "error"      - found but the move threw (detail carries the message).
 *
 * Notes:
 *   - Message-IDs are enumerated ONCE into a lookup; we never call whose() per
 *     id (that re-scans the mailbox each time and is slow over a big inbox).
 *   - The `messageId` property may or may not include angle brackets depending
 *     on the account, so both the stored value and the requested value are
 *     normalised (trimmed, surrounding <> stripped) before matching.
 *   - accountType is unreliable (Exchange and Gmail both report "imap"), so
 *     Gmail is detected via serverName() host match with an All-Mail+Bin shape
 *     fallback - identical to archive_message.js.
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
  const messageIds = args.message_ids || [];

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

  if (!Array.isArray(messageIds) || messageIds.length === 0) {
    return JSON.stringify({
      success: false,
      error: "message_ids is required and must be a non-empty array",
    });
  }

  // Normalise an RFC822 Message-ID for tolerant matching: trim whitespace and
  // strip a single pair of surrounding angle brackets if present.
  function normaliseMessageId(raw) {
    if (raw === null || raw === undefined) return "";
    let s = ("" + raw).trim();
    if (s.length >= 2 && s.charAt(0) === "<" && s.charAt(s.length - 1) === ">") {
      s = s.substring(1, s.length - 1).trim();
    }
    return s;
  }

  // Robust mailbox traversal function (reused verbatim from archive_message.js).
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

  // Detect a Gmail account (verbatim from archive_message.js).
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
  // mailboxes (verbatim from archive_message.js).
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

    // Gmail cannot be archived from Apple Mail -> whole-call hard error, touch nothing.
    if (isGmailAccount(targetAccount)) {
      return JSON.stringify({
        success: false,
        error:
          "Apple Mail cannot archive Gmail; use the gmail_archive.py IMAP tool.",
        errorCode: "GMAIL_ARCHIVE_UNSUPPORTED",
      });
    }

    // Resolve the Archive mailbox up front; never fall back to Trash/Inbox.
    const archive = findArchiveMailbox(targetAccount);
    if (!archive) {
      return JSON.stringify({
        success: false,
        error: `No "Archive" mailbox found on account "${accountName}". Cannot archive messages.`,
        errorCode: "ARCHIVE_MAILBOX_NOT_FOUND",
      });
    }

    // Locate the source mailbox
    const sourceMailbox = findMailboxByPath(targetAccount, mailboxPath);
    if (!sourceMailbox) {
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

    // Enumerate the source mailbox ONCE and build a Message-ID -> message
    // lookup in a single pass. Bulk-fetch the messageId property array rather
    // than calling .messageId() per message object.
    const msgs = sourceMailbox.messages();
    let storedIds = [];
    try {
      storedIds = sourceMailbox.messages.messageId();
    } catch (e) {
      storedIds = [];
    }

    const lookup = {}; // normalised messageId -> message object
    for (let i = 0; i < msgs.length; i++) {
      let rawId = "";
      if (storedIds && storedIds.length === msgs.length) {
        rawId = storedIds[i];
      } else {
        try {
          rawId = msgs[i].messageId();
        } catch (e) {
          rawId = "";
        }
      }
      const key = normaliseMessageId(rawId);
      if (key && !(key in lookup)) {
        lookup[key] = msgs[i];
      }
    }

    // Process each requested Message-ID against the lookup.
    const results = [];
    let archived = 0;
    let notFound = 0;
    let errored = 0;

    for (let i = 0; i < messageIds.length; i++) {
      const requested = messageIds[i];
      const key = normaliseMessageId(requested);

      if (!key) {
        notFound++;
        results.push({
          messageId: requested,
          status: "not_found",
          detail: "Empty or invalid Message-ID",
        });
        continue;
      }

      const msg = lookup[key];
      if (!msg) {
        notFound++;
        results.push({ messageId: requested, status: "not_found" });
        continue;
      }

      try {
        Mail.move(msg, { to: archive });
        // Drop from the lookup so a duplicate id in the request resolves to
        // not_found rather than attempting to move an already-moved message.
        delete lookup[key];
        archived++;
        results.push({ messageId: requested, status: "archived" });
      } catch (e) {
        errored++;
        results.push({
          messageId: requested,
          status: "error",
          detail: e.toString(),
        });
      }
    }

    return JSON.stringify({
      success: true,
      data: {
        account: accountName,
        resolvedArchive: "Archive",
        from: mailboxPath,
        archived: archived,
        not_found: notFound,
        error: errored,
        results: results,
      },
    });
  } catch (e) {
    return JSON.stringify({
      success: false,
      error: `Failed to archive messages: ${e.toString()}`,
      errorCode: "ARCHIVE_FAILED",
    });
  }
}
