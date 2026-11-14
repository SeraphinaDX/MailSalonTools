# MailSalonTools

MailSalonTools is a mailbox conversion and duplicate removal utility written in Go. It converts between EML, Maildir, mbox, and Microsoft Outlook PST while preserving attachments and message content as faithfully as the target format allows.

MailSalonTools is designed as a companion to MailSalon and MailSalonSync, but it works as a standalone command-line application.

## Status

The current development version is **0.1.0-dev**.

| Format | Read | Write |
| --- | --- | --- |
| EML | yes | yes |
| Maildir | yes | yes |
| mbox | yes | yes |
| PST | yes | yes |

PST support uses the MailSalon-maintained SeraphinaDX/outlook-pst-go fork at v0.1.10, including the folder, table, recipient, attachment, Unicode, and ANSI round-trip writer fixes.

## Usage

With command-line options, MailSalonTools runs without a TUI:

~~~sh
MailSalonTools -input ~/Maildir -output archive.pst
~~~

Formats are normally detected from the input structure and output extension. Use explicit formats when a path is ambiguous:

~~~sh
MailSalonTools -input exported-mail -input-format eml -output converted-mail -output-format maildir
~~~

Existing destinations are protected by default:

~~~sh
MailSalonTools -input mail.mbox -output archive.pst -overwrite
~~~

Supported format names are eml, maildir, mbox, and pst.

### Detect and remove duplicates

Duplicate operations run independently of conversion and support EML, Maildir, mbox, and PST. Scan a mailbox without changing it or creating output:

~~~sh
MailSalonTools -find-duplicates -input archive.mbox
MailSalonTools -find-duplicates -input ~/Maildir
MailSalonTools -find-duplicates -input archive.pst
~~~

Write a cleaned copy, keeping the first occurrence of each message:

~~~sh
MailSalonTools -remove-duplicates -input archive.mbox -output cleaned.mbox
MailSalonTools -remove-duplicates -input ~/Maildir -output ~/Maildir-cleaned
MailSalonTools -remove-duplicates -input archive.pst -output cleaned.pst
MailSalonTools -remove-duplicates -input ./exported-eml -input-format eml -output ./cleaned-eml
~~~

Removal uses the **input format by default**, without conversion. The source stays untouched; input and output must be separate, non-overlapping paths. Existing destinations require `-overwrite`; prefer a new destination for a clean archive. EML output should be a directory when retaining multiple messages.

To also convert during removal, explicitly select the output format:

~~~sh
MailSalonTools -remove-duplicates -input archive.mbox -output cleaned.pst -output-format pst
~~~

Both operations report total, unique, and duplicate message counts. Matching uses SHA-256 fingerprints of the complete RFC 822 message bytes for EML, Maildir, and mbox, normalizing CRLF to LF. Other differences in headers, MIME encoding, or whitespace remain significant. PST matching compares the available structured message fields, including bodies and attachment contents, without generating MIME boundaries. A shared Message-ID alone never establishes a duplicate; messages without Message-IDs can also match.

Matching spans folders and ignores storage folder and read-state metadata; the first copy retains its location and flags where supported. Read-state headers embedded in RFC 822 bytes remain significant. This conservative comparison can leave copies with differing delivery headers or encodings in place. Messages stream through the scan, with one fingerprint retained per unique message in memory.

### Combine mbox files

Combine two or more mbox files into one output archive. Input files are positional so shell globs work naturally:

~~~sh
MailSalonTools -combine-mbox -output combined.mbox january.mbox february.mbox march.mbox
~~~

You can also supply the first input with `-input`:

~~~sh
MailSalonTools -combine-mbox -input older.mbox -output combined.mbox newer.mbox
~~~

The operation streams messages instead of loading complete mailboxes into memory. Existing outputs are protected unless `-overwrite` is supplied.

### Split mbox by year

Split one mbox into separate files according to each message's `Date:` header:

~~~sh
MailSalonTools -split-by-year -input archive.mbox -output ./mail-by-year
~~~

Outputs are named `YYYY.mbox`, for example `2021.mbox` and `2022.mbox`. Messages with a missing or unparseable date are written to `unknown.mbox`. Existing generated year files are protected unless `-overwrite` is supplied.

Run without arguments for the small gotui wizard:

~~~sh
MailSalonTools
~~~

The TUI is intentionally minimal: enter input and output paths, see the detected formats, and press Enter.

## Preservation model

EML, Maildir, and mbox conversions carry the original RFC 822/MIME message bytes whenever possible. That avoids rebuilding attachment trees unnecessarily.

PST stores messages structurally. When PST is a source or destination, MailSalonTools maps sender, recipients, date, subject, text/HTML bodies, Message-ID, read state, folders, and attachments through a canonical message model.

PST output commits in batches so large conversions do not require every message to remain queued in one transaction.

## Building

Go 1.24 or newer:

~~~sh
make build
./MailSalonTools -version
~~~

Tests:

~~~sh
make test
~~~

## License

MailSalonTools is licensed under the GNU General Public License version 3.
