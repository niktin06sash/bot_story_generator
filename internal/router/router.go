package router

import (
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/models"
	"bot_story_generator/internal/text_messages"

	"context"
	"fmt"

	"go.uber.org/zap"
	// "go.uber.org/zap"
)

type StoryService interface {
	CreateStructuredHeroes(ctx context.Context, chatID int64) (*models.FantasyCharacters, bool)
}

type StoryRouterImpl struct {
	ctx           context.Context
	cancel        context.CancelFunc
	service       StoryService
	chan_command  chan models.Message
	chan_outbound chan models.OutboundMessage
	logger        *logger.Logger
}

func NewRouter(service StoryService, logger *logger.Logger) *StoryRouterImpl {
	context, cancel := context.WithCancel(context.Background())
	return &StoryRouterImpl{
		ctx:           context,
		cancel:        cancel,
		service:       service,
		chan_command:  make(chan models.Message, 1000),
		chan_outbound: make(chan models.OutboundMessage, 1000),
		logger:        logger,
	}
}

func (r *StoryRouterImpl) Start() {
	for {
		select {
		case <-r.ctx.Done():
			return
		case msg, ok := <-r.chan_command:
			if !ok {
				return
			}
			switch msg.Command {
			case "start":
				select {
				case <-r.ctx.Done():
					return
				case r.chan_outbound <- models.OutboundMessage{
					ChatID: msg.ChatID,
					Text:   text_messages.TextGreeting,
				}:
				}
			case "newstory":
				select {
				case <-r.ctx.Done():
					return
				case r.chan_outbound <- models.OutboundMessage{
					ChatID: msg.ChatID,
					Text:   text_messages.TextStartCreateHero,
				}:
				}

				heroes, ok := r.service.CreateStructuredHeroes(r.ctx, msg.ChatID)
				if !ok {
					r.logger.ZapLogger.Error(
						"failed to create new hero",
						zap.Int64("chatID", msg.ChatID),
					)
					select {
					case <-r.ctx.Done():
						return
					case r.chan_outbound <- models.OutboundMessage{
						ChatID: msg.ChatID,
						Text:   text_messages.TextErrorCreateHero,
					}:
					}
					continue
				}

				resp := "🌟 *Выберите своего героя из представленных вариантов:*\n\n"
				for idx, hero := range heroes.Characters {
					resp += fmt.Sprintf("🧙‍♂️ *Персонаж #%d*\n", idx+1)
					resp += "───────────────────────\n"
					if hero.Name != "" {
						resp += fmt.Sprintf("🏷️ *Имя:* %s\n", hero.Name)
					}
					if hero.Race != "" {
						resp += fmt.Sprintf("🧬 *Раса:* %s\n", hero.Race)
					}
					if hero.Class != "" {
						resp += fmt.Sprintf("⚔️ *Класс:* %s\n", hero.Class)
					}
					if hero.Appearance != "" {
						resp += fmt.Sprintf("🪞 *Внешность:* %s\n", hero.Appearance)
					}
					if len(hero.Traits) > 0 {
						resp += "💭 *Черты характера:* "
						for i, trait := range hero.Traits {
							if i > 0 {
								resp += ", "
							}
							resp += trait
						}
						resp += "\n"
					}
					if hero.Feature != "" {
						resp += fmt.Sprintf("✨ *Особенность:* %s\n", hero.Feature)
					}
					if hero.Biography != "" {
						resp += fmt.Sprintf("📜 *Биография:* %s\n", hero.Biography)
					}
					if hero.Tone != "" {
						resp += fmt.Sprintf("🎭 *Тон:* %s\n", hero.Tone)
					}
					resp += "\n───────────────────────\n\n"
				}

				select {
				case <-r.ctx.Done():
					return
				case r.chan_outbound <- models.OutboundMessage{
					ChatID: msg.ChatID,
					Text:   resp,
				}:
				}

			case "help":
				text := "Вот список команд:\n"
				for _, command := range text_messages.TextCommandForHelp {
					text += command.Command + " - " + command.Text + "\n"
				}
				select {
				case <-r.ctx.Done():
					return
				case r.chan_outbound <- models.OutboundMessage{
					ChatID: msg.ChatID,
					Text:   text,
				}:
				}
			default:
			}
		}
	}
}

func (r *StoryRouterImpl) AddComand(ctx context.Context, command string, arguments []string, chatID int64) {
	select {
	case <-r.ctx.Done():
		return
	case <-ctx.Done():
		return
	case r.chan_command <- models.Message{Command: command, Arguments: arguments, ChatID: chatID}:
	}
}

func (r *StoryRouterImpl) GetOutboundChan() chan models.OutboundMessage {
	return r.chan_outbound
}

func (r *StoryRouterImpl) CloseCommandChan() {
	close(r.chan_command)
}

func (r *StoryRouterImpl) Stop() {
	r.cancel()
	close(r.chan_outbound)
}
