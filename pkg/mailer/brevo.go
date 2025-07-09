package mailer

import (
	"bytes"
	"context"
	"fmt"
	sendinblue "github.com/sendinblue/APIv3-go-library/v2/lib"
	"gitlab.com/duel-duck/duel-duck-api/config"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	"gitlab.com/duel-duck/duel-duck-api/pkg/mtype"
	"html/template"
)

type mailer struct {
	client     *sendinblue.APIClient
	email      mtype.Email
	apiKey     string
	partnerKey string
	name       string
}

func NewMailer(c *config.Config) (Mailer, error) {
	cfg := sendinblue.NewConfiguration()
	cfg.AddDefaultHeader("api-key", c.Brevo.BrevoSecretKey)
	cfg.AddDefaultHeader("partner-key", c.Brevo.BrevoSecretKey)

	email, ok := mtype.NewEmail(c.Brevo.BrevoEmail)
	if !ok {
		return nil, apperrors.Internal("failed to init mailer")
	}

	client := sendinblue.NewAPIClient(cfg)

	return &mailer{
		client:     client,
		apiKey:     c.Brevo.BrevoSecretKey,
		partnerKey: c.Brevo.BrevoSecretKey,
		email:      email,
		name:       c.Brevo.BrevoName,
	}, nil
}

func (m *mailer) SendEmail(ctx context.Context, mail Mail) error {
	body, err := parseTemplate(mail.Template, mail.Code)
	if err != nil {
		return apperrors.Internal("failed to parse template", err)
	}

	name, ok := m.email.Username()
	if !ok {
		return apperrors.BadRequest("invalid email")
	}

	_, _, err = m.client.TransactionalEmailsApi.SendTransacEmail(ctx,
		sendinblue.SendSmtpEmail{
			Sender: &sendinblue.SendSmtpEmailSender{
				Name:  m.name,
				Email: m.email.String(),
			},
			To: []sendinblue.SendSmtpEmailTo{
				{
					Email: mail.Addressee.String(),
					Name:  name.String(),
				},
			},
			HtmlContent: body,
			Subject:     mail.Topic,
		})
	if err != nil {
		return apperrors.ServiceUnavailable("failed to send email", err)
	}

	return nil
}

func parseTemplate(templateFileName string, params ...string) (string, error) {
	templatePath := fmt.Sprintf("resources/templates/%s", templateFileName)

	t, err := template.ParseFiles(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %v", err)
	}

	buf := new(bytes.Buffer)
	tpl := make(map[string]template.HTML, len(params))

	for i, param := range params {
		s := fmt.Sprintf("code_%d", i)
		tpl[s] = template.HTML(param)
	}

	if err := t.Execute(buf, tpl); err != nil {
		return "", fmt.Errorf("failed to execute template: %v", err)
	}

	return buf.String(), nil
}

func (m *mailer) SendCode(ctx context.Context, mail Mail) error {
	body, err := parseTemplate(mail.Template, mail.Code)
	if err != nil {
		return apperrors.Internal("failed to parse template", err)
	}

	name, ok := mail.Addressee.Username()
	if !ok {
		return apperrors.BadRequest("invalid email")
	}

	_, _, err = m.client.TransactionalEmailsApi.SendTransacEmail(ctx,
		sendinblue.SendSmtpEmail{
			Sender: &sendinblue.SendSmtpEmailSender{
				Name:  m.name,
				Email: m.email.String(),
			},
			To: []sendinblue.SendSmtpEmailTo{
				{
					Email: mail.Addressee.String(),
					Name:  name.String(),
				},
			},
			HtmlContent: body,
			TextContent: mail.Code,
			Subject:     mail.Topic,
		})
	if err != nil {
		return apperrors.ServiceUnavailable("failed to send email", err)
	}

	return nil
}
