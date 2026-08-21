package service

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"strconv"
	"strings"
	"time"

	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"

	mail "github.com/wneessen/go-mail"
)

//go:embed templates/verification_code.html
var verificationCodeHTMLTmpl string

var verificationCodeTmpl = template.Must(template.New("verification_code").Parse(verificationCodeHTMLTmpl))

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

	htmlBuf := &bytes.Buffer{}
	if err := verificationCodeTmpl.Execute(htmlBuf, tmplData); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	plainBody := fmt.Sprintf("您的 %s 验证码是：%s\n\n验证码有效期：%s\n\n如果这不是您本人的操作，请忽略此邮件。\n", sceneLabel, code, cfg.EmailCodeTTL)
	subject := fmt.Sprintf("%s %s 验证码", brandName, sceneLabel)
	port, err := strconv.Atoi(cfg.SMTPPort)
	if err != nil {
		return pkg.NewAppError(pkg.EMAIL_SEND_FAILED, "invalid smtp port")
	}

	opts := []mail.Option{mail.WithPort(port)}
	if cfg.SMTPPort == "465" {
		opts = append(opts, mail.WithSSLPort(false))
	} else {
		opts = append(opts, mail.WithTLSPortPolicy(mail.TLSMandatory))
	}
	opts = append(opts,
		mail.WithSMTPAuth(mail.SMTPAuthPlain),
		mail.WithUsername(cfg.SMTPUsername),
		mail.WithPassword(cfg.SMTPPassword),
	)
	client, err := mail.NewClient(cfg.SMTPHost, opts...)
	if err != nil {
		return pkg.NewAppError(pkg.EMAIL_SEND_FAILED, err.Error())
	}

	msg := mail.NewMsg()
	if err := msg.FromFormat(brandName, cfg.SMTPFrom); err != nil {
		return pkg.NewAppError(pkg.EMAIL_SEND_FAILED, err.Error())
	}
	if err := msg.To(email); err != nil {
		return pkg.NewAppError(pkg.EMAIL_SEND_FAILED, err.Error())
	}
	msg.Subject(subject)
	msg.SetBodyString(mail.TypeTextPlain, plainBody)
	msg.AddAlternativeString(mail.TypeTextHTML, htmlBuf.String())

	if err := client.DialAndSend(msg); err != nil {
		return pkg.NewAppError(pkg.EMAIL_SEND_FAILED, err.Error())
	}
	return nil
}
