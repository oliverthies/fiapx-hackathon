package smtp

import (
	"context"
	"fmt"
	"net/smtp"

	"github.com/oliverthies/fiapx-video-processor/internal/domain"
)

type Mailer struct {
	addr   string
	from   string
	useTLS bool
}

func New(addr, from string) *Mailer {
	return &Mailer{addr: addr, from: from}
}

func (m *Mailer) JobFailed(_ context.Context, user *domain.User, job *domain.VideoJob) error {
	body := fmt.Sprintf("To: %s\r\nFrom: %s\r\nSubject: [FIAP X] Falha no processamento do vídeo\r\n\r\n"+
		"Olá,\r\n\r\nO job %s (%s) falhou.\r\nMotivo: %s\r\nCorrelation-ID: %s\r\n",
		user.Email, m.from, job.ID, job.OriginalFilename, job.ErrorMessage, job.CorrelationID)
	return smtp.SendMail(m.addr, nil, m.from, []string{user.Email}, []byte(body))
}
