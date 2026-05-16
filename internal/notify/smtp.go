package notify

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// SMTPConfig holds SMTP server connection settings.
type SMTPConfig struct {
	Enabled  bool   // Master toggle
	Host     string // SMTP server hostname
	Port     int    // SMTP port (25, 465, 587)
	User     string // SMTP username
	Pass     string // SMTP password
	From     string // Sender address
	To       string // Recipient address(es), comma-separated
	TLSMode  string // "none", "tls", "starttls"
}

// IsConfigured returns true if the minimum SMTP settings are present.
func (c *SMTPConfig) IsConfigured() bool {
	return c.Enabled && c.Host != "" && c.From != "" && c.To != ""
}

// Recipients returns the To field split into individual addresses.
func (c *SMTPConfig) Recipients() []string {
	var addrs []string
	for _, a := range strings.Split(c.To, ",") {
		a = strings.TrimSpace(a)
		if a != "" {
			addrs = append(addrs, a)
		}
	}
	return addrs
}

// SendEmail sends an email via SMTP.
// Supports three TLS modes:
//   - "none":     plain SMTP (port 25) — no encryption
//   - "starttls": EHLO then STARTTLS upgrade (port 587) — most common
//   - "tls":      implicit TLS connection (port 465) — legacy SSL
func SendEmail(cfg *SMTPConfig, subject, htmlBody string) error {
	if !cfg.IsConfigured() {
		return fmt.Errorf("smtp: not configured")
	}

	recipients := cfg.Recipients()
	if len(recipients) == 0 {
		return fmt.Errorf("smtp: no recipients")
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	// Build the email message with MIME headers
	msg := buildMessage(cfg.From, recipients, subject, htmlBody)

	switch cfg.TLSMode {
	case "tls":
		return sendImplicitTLS(addr, cfg, recipients, msg)
	case "starttls":
		return sendSTARTTLS(addr, cfg, recipients, msg)
	default: // "none"
		return sendPlain(addr, cfg, recipients, msg)
	}
}

// buildMessage constructs an RFC 2822 email with HTML content.
func buildMessage(from string, to []string, subject, htmlBody string) []byte {
	headers := fmt.Sprintf(
		"From: %s\r\n"+
			"To: %s\r\n"+
			"Subject: %s\r\n"+
			"Date: %s\r\n"+
			"MIME-Version: 1.0\r\n"+
			"Content-Type: text/html; charset=UTF-8\r\n"+
			"X-Mailer: Niyantra Dashboard\r\n"+
			"\r\n",
		from,
		strings.Join(to, ", "),
		subject,
		time.Now().Format(time.RFC1123Z),
	)
	return []byte(headers + htmlBody)
}

// sendPlain sends email without encryption (port 25).
func sendPlain(addr string, cfg *SMTPConfig, to []string, msg []byte) error {
	var auth smtp.Auth
	if cfg.User != "" {
		auth = smtp.PlainAuth("", cfg.User, cfg.Pass, cfg.Host)
	}
	return smtp.SendMail(addr, auth, cfg.From, to, msg)
}

// sendSTARTTLS connects on plain then upgrades to TLS (port 587).
func sendSTARTTLS(addr string, cfg *SMTPConfig, to []string, msg []byte) error {
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("smtp: dial %s: %w", addr, err)
	}

	c, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp: new client: %w", err)
	}
	defer c.Close()

	// EHLO
	if err := c.Hello("localhost"); err != nil {
		return fmt.Errorf("smtp: hello: %w", err)
	}

	// Upgrade to TLS
	tlsCfg := &tls.Config{
		ServerName: cfg.Host,
		MinVersion: tls.VersionTLS12,
	}
	if err := c.StartTLS(tlsCfg); err != nil {
		return fmt.Errorf("smtp: starttls: %w", err)
	}

	// Authenticate if credentials provided
	if cfg.User != "" {
		auth := smtp.PlainAuth("", cfg.User, cfg.Pass, cfg.Host)
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("smtp: auth: %w", err)
		}
	}

	return sendMailCommands(c, cfg.From, to, msg)
}

// sendImplicitTLS connects directly over TLS (port 465).
func sendImplicitTLS(addr string, cfg *SMTPConfig, to []string, msg []byte) error {
	tlsCfg := &tls.Config{
		ServerName: cfg.Host,
		MinVersion: tls.VersionTLS12,
	}

	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", addr, tlsCfg)
	if err != nil {
		return fmt.Errorf("smtp: tls dial %s: %w", addr, err)
	}

	c, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp: new client: %w", err)
	}
	defer c.Close()

	// EHLO
	if err := c.Hello("localhost"); err != nil {
		return fmt.Errorf("smtp: hello: %w", err)
	}

	// Authenticate if credentials provided
	if cfg.User != "" {
		auth := smtp.PlainAuth("", cfg.User, cfg.Pass, cfg.Host)
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("smtp: auth: %w", err)
		}
	}

	return sendMailCommands(c, cfg.From, to, msg)
}

// sendMailCommands executes the MAIL FROM, RCPT TO, DATA sequence.
func sendMailCommands(c *smtp.Client, from string, to []string, msg []byte) error {
	if err := c.Mail(from); err != nil {
		return fmt.Errorf("smtp: mail from: %w", err)
	}
	for _, addr := range to {
		if err := c.Rcpt(addr); err != nil {
			return fmt.Errorf("smtp: rcpt to %s: %w", addr, err)
		}
	}

	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp: data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("smtp: write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp: close data: %w", err)
	}

	return c.Quit()
}

// FormatQuotaAlertHTML builds a branded HTML email body for quota alerts.
func FormatQuotaAlertHTML(model string, remainingPct float64, threshold float64) string {
	severity := "warning"
	severityColor := "#f59e0b"
	if remainingPct < 5 {
		severity = "critical"
		severityColor = "#ef4444"
	} else if remainingPct < threshold/2 {
		severity = "warning"
		severityColor = "#f59e0b"
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"></head>
<body style="margin:0;padding:0;background:#0a0e17;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">
  <div style="max-width:520px;margin:20px auto;background:#111827;border:1px solid #1f2937;border-radius:12px;overflow:hidden">
    <div style="background:linear-gradient(135deg,#6366f1,#8b5cf6);padding:20px 24px;text-align:center">
      <h1 style="margin:0;color:#fff;font-size:18px;font-weight:600">⚡ Niyantra — Quota Alert</h1>
    </div>
    <div style="padding:24px">
      <div style="background:#1f2937;border-radius:8px;padding:16px;margin-bottom:16px;border-left:4px solid %s">
        <div style="color:%s;font-size:11px;text-transform:uppercase;letter-spacing:1px;font-weight:700;margin-bottom:6px">%s</div>
        <div style="color:#f3f4f6;font-size:16px;font-weight:600;margin-bottom:4px">%s</div>
        <div style="color:#9ca3af;font-size:14px">%.1f%% quota remaining</div>
      </div>
      <p style="color:#9ca3af;font-size:13px;margin:16px 0 0;line-height:1.5">
        This model has dropped below the %.0f%% alert threshold. Consider switching to an alternative model or waiting for the quota to reset.
      </p>
    </div>
    <div style="padding:12px 24px;background:#0d1117;text-align:center;border-top:1px solid #1f2937">
      <span style="color:#6b7280;font-size:11px">Niyantra — AI Operations Dashboard</span>
    </div>
  </div>
</body>
</html>`, severityColor, severityColor, severity, model, remainingPct, threshold)
}

// FormatTestEmailHTML builds a branded HTML body for the test email.
func FormatTestEmailHTML() string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"></head>
<body style="margin:0;padding:0;background:#0a0e17;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">
  <div style="max-width:520px;margin:20px auto;background:#111827;border:1px solid #1f2937;border-radius:12px;overflow:hidden">
    <div style="background:linear-gradient(135deg,#6366f1,#8b5cf6);padding:20px 24px;text-align:center">
      <h1 style="margin:0;color:#fff;font-size:18px;font-weight:600">⚡ Niyantra — Test Email</h1>
    </div>
    <div style="padding:24px;text-align:center">
      <div style="font-size:48px;margin-bottom:12px">✅</div>
      <div style="color:#f3f4f6;font-size:16px;font-weight:600;margin-bottom:8px">SMTP is working!</div>
      <div style="color:#9ca3af;font-size:14px;line-height:1.5">
        Your email notifications are properly configured.<br>
        You will receive alerts when quotas drop below the configured threshold.
      </div>
      <div style="margin-top:16px;padding:12px;background:#1f2937;border-radius:8px;color:#9ca3af;font-size:12px">
        Sent at %s
      </div>
    </div>
    <div style="padding:12px 24px;background:#0d1117;text-align:center;border-top:1px solid #1f2937">
      <span style="color:#6b7280;font-size:11px">Niyantra — AI Operations Dashboard</span>
    </div>
  </div>
</body>
</html>`, time.Now().Format("2006-01-02 15:04:05 MST"))
}
