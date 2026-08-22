// backend/internal/adapter/email.go
package adapter

import (
	"bytes"
	"context"
	"crypto/tls"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"mime/multipart"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"twitter-clone/backend/pkg/logger"
)

//go:embed templates/*.html
var emailTemplatesFS embed.FS

// EmailProvider defines the interface for email sending providers.
type EmailProvider interface {
	Send(ctx context.Context, msg *EmailMessage) error
	SendBatch(ctx context.Context, msgs []*EmailMessage) error
	SupportsAttachments() bool
	ProviderName() string
}

// EmailMessage represents an email to be sent.
type EmailMessage struct {
	To          []string
	Cc          []string
	Bcc         []string
	From        string
	FromName    string
	ReplyTo     string
	Subject     string
	TextBody    string
	HTMLBody    string
	Attachments []Attachment
	Headers     map[string]string
	Priority    int // 1 (lowest) to 5 (highest)
	Template    string
	TemplateData map[string]interface{}
}

// Attachment represents a file attachment.
type Attachment struct {
	Filename    string
	ContentType string
	Data        []byte
	Inline      bool // if true, used as inline image (cid)
}

// EmailAdapter is the main interface for sending emails.
type EmailAdapter interface {
	Send(ctx context.Context, msg *EmailMessage) error
	SendTemplate(ctx context.Context, templateName string, data map[string]interface{}, to []string, subject string) error
	SendBatch(ctx context.Context, msgs []*EmailMessage) error
	Queue(ctx context.Context, msg *EmailMessage) error
	SetProvider(provider EmailProvider)
	GetProvider() EmailProvider
	Close() error
}

// SMTPConfig holds SMTP server configuration.
type SMTPConfig struct {
	Host       string
	Port       int
	Username   string
	Password   string
	From       string
	FromName   string
	UseTLS     bool
	SkipVerify bool
	Timeout    time.Duration
	KeepAlive  bool
}

// smtpProvider implements EmailProvider using SMTP.
type smtpProvider struct {
	config     SMTPConfig
	client     *smtp.Client
	clientLock sync.Mutex
	auth       smtp.Auth
	conn       net.Conn
	log        *logrus.Entry
}

// NewSMTPProvider creates a new SMTP email provider.
func NewSMTPProvider(cfg SMTPConfig) (EmailProvider, error) {
	provider := &smtpProvider{
		config: cfg,
		log:    logger.WithField("provider", "SMTP"),
	}
	if cfg.Username != "" && cfg.Password != "" {
		provider.auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}
	return provider, nil
}

func (p *smtpProvider) ProviderName() string {
	return "SMTP"
}

func (p *smtpProvider) SupportsAttachments() bool {
	return true
}

func (p *smtpProvider) Send(ctx context.Context, msg *EmailMessage) error {
	return p.sendSMTP(ctx, msg)
}

func (p *smtpProvider) SendBatch(ctx context.Context, msgs []*EmailMessage) error {
	// For SMTP, we can send sequentially or in parallel with rate limiting.
	for _, msg := range msgs {
		if err := p.sendSMTP(ctx, msg); err != nil {
			return fmt.Errorf("batch send failed for %s: %w", msg.Subject, err)
		}
	}
	return nil
}

func (p *smtpProvider) sendSMTP(ctx context.Context, msg *EmailMessage) error {
	if len(msg.To) == 0 {
		return fmt.Errorf("no recipients")
	}
	// Build email bytes
	emailBytes, err := p.buildEmail(msg)
	if err != nil {
		return fmt.Errorf("failed to build email: %w", err)
	}
	// Get SMTP client
	client, err := p.getClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to get SMTP client: %w", err)
	}
	defer p.releaseClient()
	// Send
	from := p.config.From
	if msg.From != "" {
		from = msg.From
	}
	to := append(msg.To, msg.Cc...)
	to = append(to, msg.Bcc...)
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("MAIL command failed: %w", err)
	}
	for _, addr := range to {
		if err := client.Rcpt(addr); err != nil {
			return fmt.Errorf("RCPT command failed for %s: %w", addr, err)
		}
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA command failed: %w", err)
	}
	if _, err := writer.Write(emailBytes); err != nil {
		return fmt.Errorf("failed to write email data: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}
	p.log.WithFields(logrus.Fields{
		"to":      strings.Join(to, ","),
		"subject": msg.Subject,
	}).Debug("email sent successfully")
	return nil
}

// getClient returns an SMTP client, reusing if possible.
func (p *smtpProvider) getClient(ctx context.Context) (*smtp.Client, error) {
	p.clientLock.Lock()
	defer p.clientLock.Unlock()
	if p.client != nil && p.config.KeepAlive {
		// Test if connection is still alive
		if err := p.client.Noop(); err == nil {
			return p.client, nil
		}
		p.client.Close()
		p.client = nil
	}
	// Dial
	var conn net.Conn
	var err error
	if p.config.UseTLS {
		conn, err = tls.Dial("tcp", fmt.Sprintf("%s:%d", p.config.Host, p.config.Port), &tls.Config{
			InsecureSkipVerify: p.config.SkipVerify,
		})
	} else {
		conn, err = net.DialTimeout("tcp", fmt.Sprintf("%s:%d", p.config.Host, p.config.Port), p.config.Timeout)
	}
	if err != nil {
		return nil, fmt.Errorf("dial failed: %w", err)
	}
	client, err := smtp.NewClient(conn, p.config.Host)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create SMTP client: %w", err)
	}
	if p.auth != nil {
		if err = client.Auth(p.auth); err != nil {
			client.Close()
			return nil, fmt.Errorf("AUTH failed: %w", err)
		}
	}
	p.client = client
	return client, nil
}

func (p *smtpProvider) releaseClient() {
	p.clientLock.Lock()
	defer p.clientLock.Unlock()
	if !p.config.KeepAlive && p.client != nil {
		p.client.Quit()
		p.client = nil
	}
}

// buildEmail constructs the raw email bytes.
func (p *smtpProvider) buildEmail(msg *EmailMessage) ([]byte, error) {
	// Determine from address
	from := msg.From
	if from == "" {
		from = p.config.From
	}
	fromName := msg.FromName
	if fromName == "" {
		fromName = p.config.FromName
	}
	// Parse From
	fromAddr := mail.Address{Name: fromName, Address: from}
	fromString := fromAddr.String()
	// Build headers
	headers := make(textproto.MIMEHeader)
	headers.Set("From", fromString)
	headers.Set("To", strings.Join(msg.To, ", "))
	if len(msg.Cc) > 0 {
		headers.Set("Cc", strings.Join(msg.Cc, ", "))
	}
	if len(msg.Bcc) > 0 {
		headers.Set("Bcc", strings.Join(msg.Bcc, ", "))
	}
	headers.Set("Subject", msg.Subject)
	headers.Set("MIME-Version", "1.0")
	headers.Set("Date", time.Now().Format(time.RFC1123Z))
	if msg.ReplyTo != "" {
		headers.Set("Reply-To", msg.ReplyTo)
	}
	// Custom headers
	for k, v := range msg.Headers {
		headers.Set(k, v)
	}
	// Prepare parts
	var parts []part
	// Text part
	if msg.TextBody != "" {
		parts = append(parts, part{
			contentType: "text/plain; charset=utf-8",
			content:     []byte(msg.TextBody),
		})
	}
	if msg.HTMLBody != "" {
		parts = append(parts, part{
			contentType: "text/html; charset=utf-8",
			content:     []byte(msg.HTMLBody),
		})
	}
	// Attachments
	for _, att := range msg.Attachments {
		parts = append(parts, part{
			contentType:  att.ContentType,
			content:      att.Data,
			filename:     att.Filename,
			inline:       att.Inline,
			contentID:    att.Filename, // simple cid
		})
	}
	// Build the email
	var body bytes.Buffer
	if len(parts) == 1 && !parts[0].isAttachment() {
		// Simple single-part email
		for k, v := range headers {
			body.WriteString(k + ": " + v[0] + "\r\n")
		}
		body.WriteString("\r\n")
		body.Write(parts[0].content)
	} else {
		// Multipart
		boundary := "boundary_" + base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
		headers.Set("Content-Type", `multipart/mixed; boundary="`+boundary+`"`)
		for k, v := range headers {
			body.WriteString(k + ": " + v[0] + "\r\n")
		}
		body.WriteString("\r\n")
		for _, p := range parts {
			body.WriteString("--" + boundary + "\r\n")
			contentType := p.contentType
			if p.filename != "" {
				if p.inline {
					contentType += `; name="` + p.filename + `"`
				} else {
					contentType += `; name="` + p.filename + `"`
				}
			}
			disposition := "attachment"
			if p.inline {
				disposition = "inline"
			}
			body.WriteString("Content-Type: " + contentType + "\r\n")
			if p.filename != "" {
				body.WriteString("Content-Disposition: " + disposition + `; filename="` + p.filename + `"` + "\r\n")
				if p.inline {
					body.WriteString("Content-ID: <" + p.contentID + ">\r\n")
				}
			}
			body.WriteString("Content-Transfer-Encoding: base64\r\n")
			body.WriteString("\r\n")
			// Encode content base64
			enc := base64.NewEncoder(base64.StdEncoding, &body)
			enc.Write(p.content)
			enc.Close()
			body.WriteString("\r\n")
		}
		body.WriteString("--" + boundary + "--\r\n")
	}
	return body.Bytes(), nil
}

type part struct {
	contentType string
	content     []byte
	filename    string
	inline      bool
	contentID   string
}

func (p part) isAttachment() bool {
	return p.filename != ""
}

// defaultAdapter is the global email adapter.
var defaultAdapter EmailAdapter
var adapterOnce sync.Once

// NewEmailAdapter creates a new email adapter with the given config.
// It can accept SMTPConfig or a pre-built EmailProvider.
func NewEmailAdapter(smtpHost string, smtpPort int, smtpUser, smtpPass, smtpFrom string) (EmailAdapter, error) {
	cfg := SMTPConfig{
		Host:       smtpHost,
		Port:       smtpPort,
		Username:   smtpUser,
		Password:   smtpPass,
		From:       smtpFrom,
		UseTLS:     true,
		SkipVerify: false,
		Timeout:    30 * time.Second,
		KeepAlive:  true,
	}
	provider, err := NewSMTPProvider(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create SMTP provider: %w", err)
	}
	adapter := &emailAdapter{
		provider:       provider,
		queue:          make(chan *EmailMessage, 1000),
		workers:        5,
		rateLimiter:    rate.NewLimiter(rate.Limit(10), 20), // 10 emails/sec, burst 20
		log:            logger.WithField("component", "email_adapter"),
		stopCh:         make(chan struct{}),
		queueWaitGroup: sync.WaitGroup{},
	}
	adapter.startWorkers()
	return adapter, nil
}

// emailAdapter implements EmailAdapter with queuing and retries.
type emailAdapter struct {
	provider       EmailProvider
	queue          chan *EmailMessage
	workers        int
	rateLimiter    *rate.Limiter
	log            *logrus.Entry
	stopCh         chan struct{}
	queueWaitGroup sync.WaitGroup
	mu             sync.Mutex
	closed         bool
}

func (a *emailAdapter) SetProvider(provider EmailProvider) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.provider = provider
}

func (a *emailAdapter) GetProvider() EmailProvider {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.provider
}

func (a *emailAdapter) Send(ctx context.Context, msg *EmailMessage) error {
	if a.closed {
		return fmt.Errorf("email adapter is closed")
	}
	// Validate recipients
	if len(msg.To) == 0 {
		return fmt.Errorf("no recipients")
	}
	// Process template if specified
	if msg.Template != "" {
		if err := a.renderTemplate(msg); err != nil {
			return fmt.Errorf("template rendering failed: %w", err)
		}
	}
	// Send directly (with retries)
	return a.sendWithRetry(ctx, msg, 3)
}

func (a *emailAdapter) SendTemplate(ctx context.Context, templateName string, data map[string]interface{}, to []string, subject string) error {
	msg := &EmailMessage{
		To:           to,
		Subject:      subject,
		Template:     templateName,
		TemplateData: data,
	}
	return a.Send(ctx, msg)
}

func (a *emailAdapter) SendBatch(ctx context.Context, msgs []*EmailMessage) error {
	if a.closed {
		return fmt.Errorf("email adapter is closed")
	}
	if len(msgs) == 0 {
		return nil
	}
	// Send each with retry, but group by provider if possible.
	for _, msg := range msgs {
		if err := a.Send(ctx, msg); err != nil {
			return fmt.Errorf("batch send failed: %w", err)
		}
	}
	return nil
}

func (a *emailAdapter) Queue(ctx context.Context, msg *EmailMessage) error {
	if a.closed {
		return fmt.Errorf("email adapter is closed")
	}
	if msg == nil {
		return fmt.Errorf("message is nil")
	}
	if len(msg.To) == 0 {
		return fmt.Errorf("no recipients")
	}
	select {
	case a.queue <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("queue full, try again later")
	}
}

func (a *emailAdapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return nil
	}
	a.closed = true
	close(a.stopCh)
	a.queueWaitGroup.Wait()
	close(a.queue)
	return nil
}

func (a *emailAdapter) startWorkers() {
	for i := 0; i < a.workers; i++ {
		a.queueWaitGroup.Add(1)
		go a.worker(i)
	}
}

func (a *emailAdapter) worker(id int) {
	defer a.queueWaitGroup.Done()
	a.log.WithField("worker_id", id).Debug("email worker started")
	for {
		select {
		case msg, ok := <-a.queue:
			if !ok {
				return
			}
			// Rate limit
			if err := a.rateLimiter.Wait(context.Background()); err != nil {
				a.log.WithError(err).Error("rate limiter wait failed")
				continue
			}
			// Send with retry
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			if err := a.sendWithRetry(ctx, msg, 3); err != nil {
				a.log.WithError(err).WithFields(logrus.Fields{
					"to":      strings.Join(msg.To, ","),
					"subject": msg.Subject,
				}).Error("failed to send queued email")
			}
			cancel()
		case <-a.stopCh:
			return
		}
	}
}

func (a *emailAdapter) sendWithRetry(ctx context.Context, msg *EmailMessage, maxRetries int) error {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			time.Sleep(backoff)
			a.log.WithFields(logrus.Fields{
				"attempt": attempt + 1,
				"max":     maxRetries,
			}).Debug("retrying email send")
		}
		err := a.provider.Send(ctx, msg)
		if err == nil {
			return nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

func (a *emailAdapter) renderTemplate(msg *EmailMessage) error {
	tmpl, err := template.ParseFS(emailTemplatesFS, "templates/"+msg.Template+".html")
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}
	var htmlBuf bytes.Buffer
	if err := tmpl.Execute(&htmlBuf, msg.TemplateData); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}
	msg.HTMLBody = htmlBuf.String()
	// Optionally generate text body from HTML (simplified)
	// In production, you'd use a text-rendering template separately.
	if msg.TextBody == "" {
		// Simple fallback: strip tags (not perfect)
		text := stripTags(msg.HTMLBody)
		msg.TextBody = text
	}
	return nil
}

// stripTags removes HTML tags for plain text fallback.
func stripTags(html string) string {
	var buf bytes.Buffer
	inTag := false
	for _, ch := range html {
		if ch == '<' {
			inTag = true
			continue
		}
		if ch == '>' {
			inTag = false
			continue
		}
		if !inTag {
			buf.WriteRune(ch)
		}
	}
	return buf.String()
}

// Global functions for easy usage.

// InitEmailAdapter initializes the global email adapter.
func InitEmailAdapter(smtpHost string, smtpPort int, smtpUser, smtpPass, smtpFrom string) error {
	var err error
	adapterOnce.Do(func() {
		defaultAdapter, err = NewEmailAdapter(smtpHost, smtpPort, smtpUser, smtpPass, smtpFrom)
	})
	return err
}

// SendEmail sends an email using the global adapter.
func SendEmail(ctx context.Context, msg *EmailMessage) error {
	if defaultAdapter == nil {
		return fmt.Errorf("email adapter not initialized")
	}
	return defaultAdapter.Send(ctx, msg)
}

// SendTemplate sends a templated email.
func SendTemplate(ctx context.Context, templateName string, data map[string]interface{}, to []string, subject string) error {
	if defaultAdapter == nil {
		return fmt.Errorf("email adapter not initialized")
	}
	return defaultAdapter.SendTemplate(ctx, templateName, data, to, subject)
}

// QueueEmail queues an email for sending.
func QueueEmail(ctx context.Context, msg *EmailMessage) error {
	if defaultAdapter == nil {
		return fmt.Errorf("email adapter not initialized")
	}
	return defaultAdapter.Queue(ctx, msg)
}

// CloseEmailAdapter closes the global email adapter.
func CloseEmailAdapter() error {
	if defaultAdapter == nil {
		return nil
	}
	return defaultAdapter.Close()
}