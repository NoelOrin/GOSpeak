package service

import (
	"bytes"
	"crypto/tls"
	_ "embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/smtp"
	"strings"
	"time"

	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"
)

//go:embed templates/verification_code.html
var verificationCodeHTMLTmpl string

var sceneLabels = map[string]string{
	"register":       "注册账号",
	"reset_password": "重置密码",
	"bind_email":     "绑定邮箱",
	"change_email":   "更换邮箱",
}

type EmailService struct {
	resolveConfig func() (*config.Config, error)
}

func NewEmailService(resolveConfig func() (*config.Config, error)) *EmailService {
	return &EmailService{resolveConfig: resolveConfig}
}

func (s *EmailService) IsEnabled() bool {
	cfg, err := s.resolveConfig()
	if err != nil {
		return false
	}
	if !cfg.EmailEnabled {
		return false
	}
	return strings.TrimSpace(cfg.SMTPHost) != "" &&
		strings.TrimSpace(cfg.SMTPPort) != "" &&
		strings.TrimSpace(cfg.SMTPUsername) != "" &&
		strings.TrimSpace(cfg.SMTPPassword) != "" &&
		strings.TrimSpace(cfg.SMTPFrom) != ""
}

func (s *EmailService) SendVerificationCode(email, scene, code string) error {
	cfg, err := s.resolveConfig()
	if err != nil {
		return err
	}
	if !s.IsEnabled() {
		return pkg.NewAppError(pkg.EMAIL_NOT_CONFIGURED)
	}

	sceneLabel := sceneLabels[scene]
	if sceneLabel == "" {
		sceneLabel = scene
	}

	brandName := strings.TrimSpace(cfg.SMTPFromName)
	if brandName == "" {
		brandName = "GoSpeak"
	}

	tmplData := struct {
		Code       string
		SceneLabel string
		TTL        string
		BrandName  string
		Year       int
	}{
		Code:       code,
		SceneLabel: sceneLabel,
		TTL:        cfg.EmailCodeTTL,
		BrandName:  brandName,
		Year:       time.Now().Year(),
	}

	tmpl, err := template.New("verification_code").Parse(verificationCodeHTMLTmpl)
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	htmlBuf := &bytes.Buffer{}
	if err := tmpl.Execute(htmlBuf, tmplData); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	plainBody := fmt.Sprintf("您的 %s 验证码是：%s\n\n验证码有效期：%s\n\n如果这不是您本人的操作，请忽略此邮件。\n", sceneLabel, code, cfg.EmailCodeTTL)
	subject := fmt.Sprintf("%s %s 验证码", brandName, sceneLabel)
	message := buildMultipartMessage(cfg.SMTPFrom, cfg.SMTPFromName, email, subject, plainBody, htmlBuf.String())
	addr := fmt.Sprintf("%s:%s", cfg.SMTPHost, cfg.SMTPPort)
	auth := smtp.PlainAuth("", cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPHost)

	if cfg.SMTPPort == "465" {
		return sendMailTLS(addr, cfg.SMTPHost, auth, cfg.SMTPFrom, []string{email}, []byte(message))
	}

	if err := smtp.SendMail(addr, auth, cfg.SMTPFrom, []string{email}, []byte(message)); err != nil {
		return pkg.NewAppError(pkg.EMAIL_SEND_FAILED, err.Error())
	}
	return nil
}

func buildMultipartMessage(from, fromName, to, subject, plainBody, htmlBody string) string {
	displayFrom := from
	if strings.TrimSpace(fromName) != "" {
		displayFrom = fmt.Sprintf("%s <%s>", fromName, from)
	}
	boundary := "---mosa_boundary_9a3b7c2d"
	plainEnc := base64.StdEncoding.EncodeToString([]byte(plainBody))
	htmlEnc := base64.StdEncoding.EncodeToString([]byte(htmlBody))
	return strings.Join([]string{
		fmt.Sprintf("From: %s", displayFrom),
		fmt.Sprintf("To: %s", to),
		fmt.Sprintf("Subject: %s", subject),
		"MIME-Version: 1.0",
		fmt.Sprintf("Content-Type: multipart/alternative; boundary=%q", boundary),
		"",
		fmt.Sprintf("--%s", boundary),
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: base64",
		"",
		plainEnc,
		fmt.Sprintf("--%s", boundary),
		"Content-Type: text/html; charset=UTF-8",
		"Content-Transfer-Encoding: base64",
		"",
		htmlEnc,
		fmt.Sprintf("--%s--", boundary),
	}, "\r\n")
}

func sendMailTLS(addr, host string, auth smtp.Auth, from string, to []string, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host})
	if err != nil {
		return pkg.NewAppError(pkg.EMAIL_SEND_FAILED, err.Error())
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return pkg.NewAppError(pkg.EMAIL_SEND_FAILED, err.Error())
	}
	defer client.Close()

	if err := client.Auth(auth); err != nil {
		return pkg.NewAppError(pkg.EMAIL_SEND_FAILED, err.Error())
	}
	if err := client.Mail(from); err != nil {
		return pkg.NewAppError(pkg.EMAIL_SEND_FAILED, err.Error())
	}
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return pkg.NewAppError(pkg.EMAIL_SEND_FAILED, err.Error())
		}
	}
	w, err := client.Data()
	if err != nil {
		return pkg.NewAppError(pkg.EMAIL_SEND_FAILED, err.Error())
	}
	if _, err := w.Write(msg); err != nil {
		return pkg.NewAppError(pkg.EMAIL_SEND_FAILED, err.Error())
	}
	if err := w.Close(); err != nil {
		return pkg.NewAppError(pkg.EMAIL_SEND_FAILED, err.Error())
	}
	if err := client.Quit(); err != nil {
		return pkg.NewAppError(pkg.EMAIL_SEND_FAILED, err.Error())
	}
	return nil
}
