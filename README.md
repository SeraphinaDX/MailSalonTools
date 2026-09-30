# MailSalonTools

MailSalonTools is a mailbox conversion utility written in Go. It converts between EML, Maildir, mbox, and Microsoft Outlook PST while preserving attachments and message content as faithfully as the target format allows.

MailSalonTools is designed as a companion to MailSalon and MailSalonSync, but it works as a standalone command-line application.

## Status

The current development version is **0.1.0-dev**.

| Format | Read | Write |
| --- | --- | --- |
| EML | yes | yes |
| Maildir | yes | yes |
| mbox | yes | yes |
| PST | yes | yes |

PST support uses the MailSalon-maintained SeraphinaDX/outlook-pst-go fork at v0.1.3, including the folder, table, recipient, attachment, Unicode, and ANSI round-trip writer fixes.

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
