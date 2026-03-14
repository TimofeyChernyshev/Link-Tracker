package bot

import (
	"context"
	"fmt"
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

type Client struct {
	api        *tgbotapi.BotAPI
	dispatcher CommandDispatcher

	stopChan chan struct{}
	workerWg sync.WaitGroup
	senderWg sync.WaitGroup

	jobs     chan tgbotapi.Update
	outgoing chan *domain.Response
}

func NewClient(token string, d CommandDispatcher) (*Client, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("cannot create bot apiL %w", err)
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

	return &Client{
		api:        api,
		dispatcher: d,
	}, nil
}

func (c *Client) Start() error {
	c.stopChan = make(chan struct{})
	c.jobs = make(chan tgbotapi.Update, jobsBufferSize)
	c.outgoing = make(chan *domain.Response, outgoingBufferSize)

	for range workers {
		c.workerWg.Add(1)
		go c.worker()
	}

	for range senders {
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

	response, err := c.dispatcher.Dispatch(msg)
	if err != nil {
		slog.Error("error ocurs while trying to dispatch message", "error", err, "message", msg.Text)
		c.SendMessage(&domain.Response{Text: err.Error(), ChatID: msg.ChatID})
		return
	}

	if response != nil {
		slog.Debug("got response", "response text", response.Text, "chatID", response.ChatID)
		c.SendMessage(response)
	}
}
