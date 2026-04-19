package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type Client struct {
	api *tgbotapi.BotAPI

	stopChan chan struct{}
	workerWg sync.WaitGroup
	senderWg sync.WaitGroup

	jobs     chan tgbotapi.Update
	outgoing chan *domain.Response

	incoming chan *domain.Message

	workers            int
	senders            int
	jobsBufferSize     int
	outgoingBufferSize int
}

func NewClient(token string, endpoint string, workers, senders, jobsBufferSize, outgoingBufferSize int) (*Client, error) {
	var (
		api *tgbotapi.BotAPI
		err error
	)

	// для интеграционных тестов
	if endpoint != "" {
		api, err = tgbotapi.NewBotAPIWithAPIEndpoint(token, endpoint+"/bot%s/%s")
	} else {
		api, err = tgbotapi.NewBotAPI(token)
	}
	if err != nil {
		return nil, fmt.Errorf("cannot create bot api: %w", err)
	}

	return &Client{
		api:                api,
		incoming:           make(chan *domain.Message, jobsBufferSize),
		workers:            workers,
		senders:            senders,
		jobsBufferSize:     jobsBufferSize,
		outgoingBufferSize: outgoingBufferSize,
	}, nil
}

func (c *Client) SetCommands(cmds []domain.BotCommand) {
	tgCommands := make([]tgbotapi.BotCommand, 0, len(cmds))
	for _, c := range cmds {
		tgCommands = append(tgCommands, tgbotapi.BotCommand{Command: c.Name, Description: c.Description})
	}

	setCmdCfg := tgbotapi.NewSetMyCommands(tgCommands...)
	_, err := c.api.Request(setCmdCfg)
	if err != nil {
		slog.Warn("failed to set commands menu", "error", err)
	}
}

func (c *Client) Start() error {
	c.stopChan = make(chan struct{})
	c.jobs = make(chan tgbotapi.Update, c.jobsBufferSize)
	c.outgoing = make(chan *domain.Response, c.outgoingBufferSize)

	for range c.workers {
		c.workerWg.Add(1)
		go c.worker()
	}

	for range c.senders {
		c.senderWg.Add(1)
		go c.sender()
	}

	slog.Info("Authorized on account", "account", c.api.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	updates := c.api.GetUpdatesChan(u)

	for {
		select {
		case <-c.stopChan:
			close(c.jobs)
			slog.Info("stopping update channel")
			return nil
		case update, ok := <-updates:
			if !ok {
				close(c.jobs)
				slog.Info("updates channel closed")
				return nil
			}

			if update.Message != nil {
				c.jobs <- update
			}
		}
	}
}

// SendMessage отправляет в канал сообщений для отправки сообщения из внешнего источника
func (c *Client) SendMessage(resp *domain.Response) {
	select {
	case <-c.stopChan:
		slog.Info("stopping send channel")
	case c.outgoing <- resp:
	default:
		slog.Warn("outgoing queue full")
	}
}

func (c *Client) Receive(ctx context.Context) (*domain.Message, error) {
	select {
	case msg := <-c.incoming:
		return msg, nil
	case <-c.stopChan:
		return nil, errors.New("bot stopped")
	case <-ctx.Done():
		return nil, fmt.Errorf("receiving message: %w", ctx.Err())
	}
}

// Stop реализует graceful shutdown
func (c *Client) Stop(ctx context.Context) error {
	close(c.stopChan)

	c.api.StopReceivingUpdates()

	workerDone := make(chan struct{})
	go func() {
		c.workerWg.Wait()
		close(workerDone)
	}()

	select {
	case <-workerDone:
		slog.Info("all handlers finished")
	case <-ctx.Done():
		return fmt.Errorf("wait for workers: %w", ctx.Err())
	}

	close(c.outgoing)

	senderDone := make(chan struct{})
	go func() {
		c.senderWg.Wait()
		close(senderDone)
	}()

	select {
	case <-senderDone:
		slog.Info("all senders finished")
	case <-ctx.Done():
		return fmt.Errorf("wait for senders: %w", ctx.Err())
	}

	close(c.incoming)

	return nil
}

func (c *Client) worker() {
	defer c.workerWg.Done()

	for upd := range c.jobs {
		c.handleUpdate(upd)
	}
}

func (c *Client) sender() {
	defer c.senderWg.Done()

	for resp := range c.outgoing {
		msg := tgbotapi.NewMessage(resp.ChatID, resp.Text)

		_, err := c.api.Send(msg)
		if err != nil {
			slog.Warn("Error sending message", "err", err)
		}

		slog.Debug("message sent to chat", "chatID", msg.ChatID)
	}
}

func (c *Client) handleUpdate(update tgbotapi.Update) {
	msg := &domain.Message{
		Text:      update.Message.Text,
		ChatID:    update.Message.Chat.ID,
		Username:  update.Message.From.UserName,
		MessageID: update.Message.MessageID,
	}

	slog.Debug("got message", "message text", msg.Text, "chatID", msg.ChatID, "user", msg.Username)

	select {
	case c.incoming <- msg:
	default:
		slog.Warn("incoming queue full")
	}
}
