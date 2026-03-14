package bot

import (
	"context"
	"log/slog"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

const (
	workers = 8
	senders = 4

	jobsBufferSize     = 100
	outgoingBufferSize = 100
)

type BotClient struct {
	api        *tgbotapi.BotAPI
	dispatcher CommandDispatcher

	stopChan chan struct{}
	workerWg sync.WaitGroup
	senderWg sync.WaitGroup

	jobs     chan tgbotapi.Update
	outgoing chan *domain.Response
}

func NewBotClient(token string, d CommandDispatcher) (*BotClient, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	cmds := d.GetCommands()

	tgCommands := make([]tgbotapi.BotCommand, 0, len(cmds))
	for _, c := range cmds {
		tgCommands = append(tgCommands, tgbotapi.BotCommand{Command: c.Name, Description: c.Description})
	}

	setCmdCfg := tgbotapi.NewSetMyCommands(tgCommands...)
	_, err = api.Request(setCmdCfg)
	if err != nil {
		slog.Warn("failed to set commands menu", "error", err)
	}

	return &BotClient{
		api:        api,
		dispatcher: d,
	}, nil
}

func (b *BotClient) Start() error {
	b.stopChan = make(chan struct{})
	b.jobs = make(chan tgbotapi.Update, jobsBufferSize)
	b.outgoing = make(chan *domain.Response, outgoingBufferSize)

	for range workers {
		b.workerWg.Add(1)
		go b.worker()
	}

	for range senders {
		b.senderWg.Add(1)
		go b.sender()
	}

	slog.Info("Authorized on account", "account", b.api.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	updates := b.api.GetUpdatesChan(u)

	for {
		select {
		case <-b.stopChan:
			close(b.jobs)
			slog.Info("stopping update channel")
			return nil
		case update, ok := <-updates:
			if !ok {
				close(b.jobs)
				slog.Info("updates channel closed")
				return nil
			}

			if update.Message != nil {
				b.jobs <- update
			}
		}
	}
}

// SendMessage отправляет в канал сообщений для отправки сообщения из внешнего источника
func (b *BotClient) SendMessage(resp *domain.Response) {
	select {
	case <-b.stopChan:
		slog.Info("stopping send channel")
	case b.outgoing <- resp:
	default:
		slog.Warn("outgoing queue full")
	}
}

// Stop реализует graceful shutdown
func (b *BotClient) Stop(ctx context.Context) error {
	close(b.stopChan)

	b.api.StopReceivingUpdates()

	workerDone := make(chan struct{})
	go func() {
		b.workerWg.Wait()
		close(workerDone)
	}()

	select {
	case <-workerDone:
		slog.Info("all handlers finished")
	case <-ctx.Done():
		return ctx.Err()
	}

	close(b.outgoing)

	senderDone := make(chan struct{})
	go func() {
		b.senderWg.Wait()
		close(senderDone)
	}()

	select {
	case <-senderDone:
		slog.Info("all senders finished")
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

func (b *BotClient) worker() {
	defer b.workerWg.Done()

	for upd := range b.jobs {
		b.handleUpdate(upd)
	}
}

func (b *BotClient) sender() {
	defer b.senderWg.Done()

	for resp := range b.outgoing {
		msg := tgbotapi.NewMessage(resp.ChatID, resp.Text)

		_, err := b.api.Send(msg)
		if err != nil {
			slog.Warn("Error sending message", "err", err)
		}

		slog.Debug("message sent to chat", "chatID", msg.ChatID)
	}
}

func (b *BotClient) handleUpdate(update tgbotapi.Update) {
	msg := &domain.Message{
		Text:      update.Message.Text,
		ChatID:    update.Message.Chat.ID,
		Username:  update.Message.From.UserName,
		MessageID: update.Message.MessageID,
	}

	slog.Debug("got message", "message text", msg.Text, "chatID", msg.ChatID, "user", msg.Username)

	response, err := b.dispatcher.Dispatch(msg)
	if err != nil {
		slog.Error("error ocurs while trying to dispatch message", "error", err, "message", msg.Text)
		b.SendMessage(&domain.Response{Text: err.Error(), ChatID: msg.ChatID})
		return
	}

	if response != nil {
		slog.Debug("got response", "response text", response.Text, "chatID", response.ChatID)
		b.SendMessage(response)
	}
}
