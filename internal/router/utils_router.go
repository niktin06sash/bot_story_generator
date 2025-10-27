package router

import (
	"bot_story_generator/internal/models"
	"context"
)

func (r *StoryRouterImpl) createOutboundMessage(ctx context.Context, userID int64, text string, butargs ...models.ButtonArg) {
	select {
	case <-r.ctx.Done():
		return
	case r.chan_outbound <- models.NewOutboundMessage(ctx, userID, text, butargs...):
	}
}
func (r *StoryRouterImpl) createEditMessage(userID int64, msgID int, text string, butargs ...models.ButtonArg) {
	select {
	case <-r.ctx.Done():
		return
	case r.chan_edit <- models.NewEditMessage(userID, msgID, text, butargs...):
	}
}
func (r *StoryRouterImpl) createDeleteMessage(userID int64, msgID int) {
	select {
	case <-r.ctx.Done():
		return
	case r.chan_edit <- models.NewDeleteMessage(userID, msgID):
	}
}
func (r *StoryRouterImpl) cleanUserState(userID int64) {
	r.mux.Lock()
	delete(r.userState, userID)
	r.mux.Unlock()
}
