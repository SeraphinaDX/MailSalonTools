package mailbox

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/SeraphinaDX/MailSalonTools/internal/model"
	outlookpst "github.com/grokify/outlook-pst-go"
	"github.com/grokify/outlook-pst-go/pkg/disk"
	"github.com/grokify/outlook-pst-go/pkg/ltp"
)

type pstEntry struct {
	folder string
	msg    *outlookpst.Message
}

type pstReader struct {
	pst     *outlookpst.PST
	entries []pstEntry
	index   int
}

func newPSTReader(filename string) (*pstReader, error) {
	pst, err := outlookpst.Open(filename)
	if err != nil {
		return nil, err
	}
	root, err := pst.RootFolder()
	if err != nil {
		_ = pst.Close()
		return nil, err
	}
	r := &pstReader{pst: pst}
	r.collect(root, "")
	return r, nil
}

func (r *pstReader) collect(folder *outlookpst.Folder, prefix string) {
	for msg, err := range folder.Messages() {
		if err != nil {
			break
		}
		r.entries = append(r.entries, pstEntry{folder: prefix, msg: msg})
	}
	for child, err := range folder.Subfolders() {
		if err != nil {
			break
		}
		name, _ := child.Name()
		next := name
		if prefix != "" {
			next = prefix + "/" + name
		}
		r.collect(child, next)
	}
}

func (r *pstReader) Next() (*model.Message, error) {
	if r.index >= len(r.entries) {
		return nil, io.EOF
	}
	entry := r.entries[r.index]
	r.index++
	pm := entry.msg
	m := &model.Message{Folder: cleanFolder(entry.folder), Parsed: true}

	m.Subject, _ = pm.Subject()
	m.MessageID, _ = pm.InternetMessageID()
	m.TextBody, _ = pm.Body()
	if html, htmlErr := pm.HTMLBody(); htmlErr == nil {
		m.HTMLBody = html
	} else if !strings.Contains(htmlErr.Error(), "property not found") {
		return nil, fmt.Errorf("read PST HTML body: %w", htmlErr)
	}
	m.From.Name, _ = pm.SenderName()
	m.From.Email, _ = pm.SenderEmail()
	if t, err := pm.SubmitTime(); err == nil {
		m.Date = t
	} else if t, err := pm.DeliveryTime(); err == nil {
		m.Date = t
	}

	for recipient, err := range pm.Recipients() {
		if err != nil {
			break
		}
		name, _ := recipient.Name()
		email, _ := recipient.Email()
		rt, _ := recipient.Type()
		addr := model.Address{Name: name, Email: email}
		switch rt {
		case outlookpst.RecipientTo:
			m.To = append(m.To, addr)
		case outlookpst.RecipientCc:
			m.Cc = append(m.Cc, addr)
		case outlookpst.RecipientBcc:
			m.Bcc = append(m.Bcc, addr)
		}
	}

	for attachment, err := range pm.Attachments() {
		if err != nil {
			break
		}
		name, _ := attachment.Filename()
		mimeType, _ := attachment.MimeType()
		contentID, _ := attachment.ContentID()
		data, err := attachment.Data()
		if err != nil {
			continue
		}
		m.Attachments = append(m.Attachments, model.Attachment{
			Filename: name, MIMEType: mimeType, ContentID: contentID, Data: data,
		})
	}
	return m, nil
}

func (r *pstReader) Close() error { return r.pst.Close() }

type pstWriter struct {
	filename string
	pst      *outlookpst.PST
	ctx      *outlookpst.WriteContext
	root     *outlookpst.Folder
	folders  map[string]*outlookpst.Folder
	batch    int
}

func newPSTWriter(filename string, overwrite bool) (*pstWriter, error) {
	if _, err := os.Stat(filename); err == nil {
		if !overwrite {
			return nil, fmt.Errorf("output exists: %s (use -overwrite)", filename)
		}
		if err := os.Remove(filename); err != nil {
			return nil, err
		}
	}
	pst, err := outlookpst.Create(filename, disk.FormatUnicode)
	if err != nil {
		return nil, err
	}
	w := &pstWriter{filename: filename, pst: pst}
	if err := w.begin(); err != nil {
		_ = pst.Close()
		return nil, err
	}
	return w, nil
}

func (w *pstWriter) begin() error {
	root, err := w.pst.RootFolder()
	if err != nil {
		return err
	}
	ctx, err := w.pst.BeginWrite()
	if err != nil {
		return err
	}
	w.root, w.ctx = root, ctx
	w.folders = map[string]*outlookpst.Folder{"": root}
	w.indexFolders(root, "")
	w.batch = 0
	return nil
}

func (w *pstWriter) indexFolders(folder *outlookpst.Folder, prefix string) {
	for child, err := range folder.Subfolders() {
		if err != nil {
			break
		}
		name, _ := child.Name()
		next := name
		if prefix != "" {
			next = prefix + "/" + name
		}
		w.folders[next] = child
		w.indexFolders(child, next)
	}
}

func (w *pstWriter) folderFor(folder string) (*outlookpst.Folder, error) {
	folder = cleanFolder(folder)
	if folder == "" {
		folder = "Imported"
	}
	if f, ok := w.folders[folder]; ok {
		return f, nil
	}
	current := w.root
	currentPath := ""
	for _, component := range strings.Split(folder, "/") {
		next := component
		if currentPath != "" {
			next = currentPath + "/" + component
		}
		if f, ok := w.folders[next]; ok {
			current = f
			currentPath = next
			continue
		}
		created, err := w.ctx.CreateFolder(current, component)
		if err != nil {
			return nil, err
		}
		w.folders[next] = created
		current, currentPath = created, next
	}
	return current, nil
}

func (w *pstWriter) Write(m *model.Message) error {
	if err := m.EnsureParsed(); err != nil {
		return err
	}
	folder, err := w.folderFor(m.Folder)
	if err != nil {
		return err
	}

	b := w.ctx.CreateMessage(folder).
		SetSubject(m.Subject).
		SetBody(m.TextBody).
		SetHTMLBody(m.HTMLBody)
	if m.From.Email != "" {
		b.SetFrom(m.From.Name, m.From.Email)
	}
	if !m.Date.IsZero() {
		b.SetSentTime(m.Date)
	}
	for _, a := range m.To {
		b.AddTo(a.Name, a.Email)
	}
	for _, a := range m.Cc {
		b.AddCC(a.Name, a.Email)
	}
	for _, a := range m.Bcc {
		b.AddBCC(a.Name, a.Email)
	}
	for _, a := range m.Attachments {
		b.AddAttachmentWithMime(a.Filename, a.Data, a.MIMEType)
	}
	if m.MessageID != "" {
		b.SetProperty(ltp.PidTagInternetMessageId, m.MessageID)
	}

	flags := int32(0)
	if strings.Contains(m.Flags, "S") {
		flags |= 1
	}
	if len(m.Attachments) > 0 {
		flags |= 0x10
	}
	b.SetProperty(ltp.PidTagMessageFlags, flags)
	if _, err := b.Build(); err != nil {
		return err
	}

	w.batch++
	if w.batch >= 100 {
		return w.flushAndContinue()
	}
	return nil
}

func (w *pstWriter) flushAndContinue() error {
	if err := w.ctx.Commit(); err != nil {
		return err
	}
	if err := w.pst.Close(); err != nil {
		return err
	}
	pst, err := outlookpst.OpenReadWrite(w.filename)
	if err != nil {
		return err
	}
	w.pst = pst
	return w.begin()
}

func (w *pstWriter) Close() error {
	if w.ctx != nil {
		if err := w.ctx.Commit(); err != nil {
			_ = w.pst.Close()
			return err
		}
		w.ctx = nil
	}
	return w.pst.Close()
}
