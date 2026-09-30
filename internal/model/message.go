package model

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"strings"
	"time"
)

type Address struct {
	Name  string
	Email string
}

type Attachment struct {
	Filename  string
	MIMEType  string
	ContentID string
	Data      []byte
}

type Message struct {
	Raw         []byte
	Folder      string
	Flags       string
	Subject     string
	MessageID   string
	From        Address
	To          []Address
	Cc          []Address
	Bcc         []Address
	Date        time.Time
	TextBody    string
	HTMLBody    string
	Attachments []Attachment
	Parsed      bool
}

func (m *Message) EnsureParsed() error {
	if m.Parsed || len(m.Raw) == 0 {
		m.Parsed = true
		return nil
	}
	folder, flags := m.Folder, m.Flags
	parsed, err := ParseRFC822(m.Raw)
	if err != nil {
		return err
	}
	parsed.Folder = folder
	parsed.Flags = flags
	*m = *parsed
	return nil
}

func (m *Message) RFC822() ([]byte, error) {
	if len(m.Raw) > 0 {
		return append([]byte(nil), m.Raw...), nil
	}
	return BuildRFC822(m)
}

func ParseRFC822(raw []byte) (*Message, error) {
	rm, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("parse RFC 822 message: %w", err)
	}
	out := &Message{Raw: append([]byte(nil), raw...), Parsed: true}
	out.Subject = decodeHeader(rm.Header.Get("Subject"))
	out.MessageID = strings.TrimSpace(rm.Header.Get("Message-ID"))
	out.From = firstAddress(rm.Header.Get("From"))
	out.To = parseAddresses(rm.Header.Get("To"))
	out.Cc = parseAddresses(rm.Header.Get("Cc"))
	out.Bcc = parseAddresses(rm.Header.Get("Bcc"))
	if d, err := mail.ParseDate(rm.Header.Get("Date")); err == nil {
		out.Date = d
	}
	if err := parseEntity(rm.Body, textproto.MIMEHeader(rm.Header), out); err != nil {
		return nil, err
	}
	return out, nil
}

func parseEntity(r io.Reader, h textproto.MIMEHeader, out *Message) error {
	mediaType, params, _ := mime.ParseMediaType(h.Get("Content-Type"))
	if mediaType == "" {
		mediaType = "text/plain"
	}
	if strings.HasPrefix(strings.ToLower(mediaType), "multipart/") {
		boundary := params["boundary"]
		if boundary == "" {
			return fmt.Errorf("multipart message has no boundary")
		}
		mr := multipart.NewReader(r, boundary)
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return fmt.Errorf("read MIME part: %w", err)
			}
			if err := parseEntity(part, part.Header, out); err != nil {
				_ = part.Close()
				return err
			}
			_ = part.Close()
		}
	}
	body, err := io.ReadAll(decodeTransfer(r, h.Get("Content-Transfer-Encoding")))
	if err != nil {
		return fmt.Errorf("read MIME body: %w", err)
	}
	disposition, dispParams, _ := mime.ParseMediaType(h.Get("Content-Disposition"))
	filename := dispParams["filename"]
	if filename == "" {
		_, ctParams, _ := mime.ParseMediaType(h.Get("Content-Type"))
		filename = ctParams["name"]
	}
	filename = decodeHeader(filename)
	if strings.EqualFold(disposition, "attachment") || filename != "" {
		if mediaType == "" {
			mediaType = "application/octet-stream"
		}
		out.Attachments = append(out.Attachments, Attachment{
			Filename: filename, MIMEType: mediaType,
			ContentID: strings.Trim(h.Get("Content-ID"), "<>"), Data: body,
		})
		return nil
	}
	switch strings.ToLower(mediaType) {
	case "text/html":
		if out.HTMLBody != "" {
			out.HTMLBody += "
"
		}
		out.HTMLBody += string(body)
	default:
		if strings.HasPrefix(strings.ToLower(mediaType), "text/") {
			if out.TextBody != "" {
				out.TextBody += "
"
			}
			out.TextBody += string(body)
		}
	}
	return nil
}

func decodeTransfer(r io.Reader, encoding string) io.Reader {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "base64":
		return base64.NewDecoder(base64.StdEncoding, r)
	case "quoted-printable":
		return quotedprintable.NewReader(r)
	default:
		return r
	}
}

func BuildRFC822(m *Message) ([]byte, error) {
	var out bytes.Buffer
	writeHeader := func(name, value string) {
		if strings.TrimSpace(value) != "" {
			fmt.Fprintf(&out, "%s: %s
", name, value)
		}
	}
	writeHeader("From", formatAddress(m.From))
	writeHeader("To", formatAddresses(m.To))
	writeHeader("Cc", formatAddresses(m.Cc))
	writeHeader("Bcc", formatAddresses(m.Bcc))
	writeHeader("Subject", encodeHeader(m.Subject))
	if !m.Date.IsZero() {
		writeHeader("Date", m.Date.Format(time.RFC1123Z))
	}
	writeHeader("Message-ID", m.MessageID)
	writeHeader("MIME-Version", "1.0")

	if len(m.Attachments) == 0 && m.HTMLBody == "" {
		writeHeader("Content-Type", "text/plain; charset=utf-8")
		writeHeader("Content-Transfer-Encoding", "quoted-printable")
		out.WriteString("
")
		qp := quotedprintable.NewWriter(&out)
		_, _ = io.WriteString(qp, m.TextBody)
		_ = qp.Close()
		return out.Bytes(), nil
	}

	if len(m.Attachments) == 0 {
		contentType, body, err := alternativeBody(m)
		if err != nil {
			return nil, err
		}
		writeHeader("Content-Type", contentType)
		out.WriteString("
")
		out.Write(body)
		return out.Bytes(), nil
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	writeHeader("Content-Type", mime.FormatMediaType("multipart/mixed", map[string]string{"boundary": mw.Boundary()}))
	out.WriteString("
")
	if m.TextBody != "" || m.HTMLBody != "" {
		var contentType string
		var payload []byte
		var err error
		if m.HTMLBody != "" {
			contentType, payload, err = alternativeBody(m)
		} else {
			contentType = "text/plain; charset=utf-8"
			payload = quotedPrintableBytes(m.TextBody)
		}
		if err != nil {
			return nil, err
		}
		h := make(textproto.MIMEHeader)
		h.Set("Content-Type", contentType)
		if m.HTMLBody == "" {
			h.Set("Content-Transfer-Encoding", "quoted-printable")
		}
		part, _ := mw.CreatePart(h)
		_, _ = part.Write(payload)
	}
	for _, a := range m.Attachments {
		ct := a.MIMEType
		if ct == "" {
			ct = "application/octet-stream"
		}
		h := make(textproto.MIMEHeader)
		h.Set("Content-Type", mime.FormatMediaType(ct, map[string]string{"name": a.Filename}))
		h.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": a.Filename}))
		h.Set("Content-Transfer-Encoding", "base64")
		if a.ContentID != "" {
			h.Set("Content-ID", "<"+a.ContentID+">")
		}
		part, _ := mw.CreatePart(h)
		writeBase64(part, a.Data)
	}
	_ = mw.Close()
	out.Write(body.Bytes())
	return out.Bytes(), nil
}

func alternativeBody(m *Message) (string, []byte, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if m.TextBody != "" {
		h := make(textproto.MIMEHeader)
		h.Set("Content-Type", "text/plain; charset=utf-8")
		h.Set("Content-Transfer-Encoding", "quoted-printable")
		p, _ := mw.CreatePart(h)
		_, _ = p.Write(quotedPrintableBytes(m.TextBody))
	}
	if m.HTMLBody != "" {
		h := make(textproto.MIMEHeader)
		h.Set("Content-Type", "text/html; charset=utf-8")
		h.Set("Content-Transfer-Encoding", "quoted-printable")
		p, _ := mw.CreatePart(h)
		_, _ = p.Write(quotedPrintableBytes(m.HTMLBody))
	}
	if err := mw.Close(); err != nil {
		return "", nil, err
	}
	return mime.FormatMediaType("multipart/alternative", map[string]string{"boundary": mw.Boundary()}), body.Bytes(), nil
}

func quotedPrintableBytes(s string) []byte {
	var b bytes.Buffer
	qp := quotedprintable.NewWriter(&b)
	_, _ = io.WriteString(qp, s)
	_ = qp.Close()
	return b.Bytes()
}

func writeBase64(w io.Writer, data []byte) {
	encoded := base64.StdEncoding.EncodeToString(data)
	for len(encoded) > 76 {
		_, _ = io.WriteString(w, encoded[:76]+"
")
		encoded = encoded[76:]
	}
	if encoded != "" {
		_, _ = io.WriteString(w, encoded+"
")
	}
}

func firstAddress(value string) Address {
	list := parseAddresses(value)
	if len(list) == 0 {
		return Address{}
	}
	return list[0]
}

func parseAddresses(value string) []Address {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	list, err := mail.ParseAddressList(value)
	if err != nil {
		return nil
	}
	out := make([]Address, 0, len(list))
	for _, a := range list {
		out = append(out, Address{Name: a.Name, Email: a.Address})
	}
	return out
}

func formatAddress(a Address) string {
	if a.Email == "" {
		return ""
	}
	return (&mail.Address{Name: a.Name, Address: a.Email}).String()
}

func formatAddresses(in []Address) string {
	values := make([]string, 0, len(in))
	for _, a := range in {
		if s := formatAddress(a); s != "" {
			values = append(values, s)
		}
	}
	return strings.Join(values, ", ")
}

func decodeHeader(s string) string {
	if s == "" {
		return ""
	}
	v, err := new(mime.WordDecoder).DecodeHeader(s)
	if err != nil {
		return s
	}
	return v
}

func encodeHeader(s string) string {
	if s == "" {
		return ""
	}
	for _, r := range s {
		if r > 127 {
			return mime.QEncoding.Encode("utf-8", s)
		}
	}
	return s
}
