package router

import (
	"bot_story_generator/internal/models"
	"context"
)

func (r *StoryRouterImpl) createOutboundMessage(ctx context.Context, chatID int64, text string, butargs ...models.ButtonArg) {
	select {
	case <-r.ctx.Done():
		return
	case r.chan_outbound <- models.NewOutboundMessage(ctx, chatID, text, butargs...):
	}
}
func (r *StoryRouterImpl) cleanUserState(chatID int64) {
	r.mux.Lock()
	delete(r.userState, chatID)
	r.mux.Unlock()
}
