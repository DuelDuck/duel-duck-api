package mailer

import (
	"context"
	"gitlab.com/duel-duck/duel-duck-api/pkg/mtype"
)

type Mailer interface {
	SendEmail(ctx context.Context, mail Mail) error
}

type Mail struct {
	Addressee mtype.Email
	Template  string
	Topic     string
	Code      string
}

func NewMail(addressee mtype.Email, template string, topic string, code string) Mail {
	return Mail{
		Addressee: addressee,
		Template:  template,
		Topic:     topic,
		Code:      code,
	}
}
