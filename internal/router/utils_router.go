package router

import (
	"bot_story_generator/internal/models"
	"context"
)

func (r *StoryRouterImpl) createOutboundMessage(ctx context.Context, userID int64, text string, trace models.Trace, butargs ...models.ButtonArg) {
	select {
	case <-r.ctx.Done():
		return
	case r.chan_outbound <- models.NewOutboundMessage(ctx, userID, text, trace, butargs...):
	}
}
func (r *StoryRouterImpl) createEditMessage(userID int64, msgID int, text string, trace models.Trace, butargs ...models.ButtonArg) {
	select {
	case <-r.ctx.Done():
		return
	case r.chan_edit <- models.NewEditMessage(userID, msgID, text, trace, butargs...):
	}
}
func (r *StoryRouterImpl) createDeleteMessage(userID int64, msgID int, trace models.Trace) {
	select {
	case <-r.ctx.Done():
		return
	case r.chan_delete <- models.NewDeleteMessage(userID, msgID, trace):
	}
}
func (r *StoryRouterImpl) createInvoiceMessage(sub *models.Subscription, trace models.Trace) {
	select {
	case <-r.ctx.Done():
		return
	case r.chan_bot_invoice <- models.NewInvoiceMessage(sub, trace):
	}
}
func (r *StoryRouterImpl) createPaymentMessage(pm *models.PaymentData) {
	select {
	case <-r.ctx.Done():
		return
	case r.chan_outbound_payments <- pm:
	}
}
func (r *StoryRouterImpl) cleanUserState(userID int64) {
	r.mux.Lock()
	delete(r.userState, userID)
	r.mux.Unlock()
}

func (r *StoryRouterImpl) checkAdmin(userID int64) bool {
	r.mux.RLock()
	_, ok := r.admins[userID]
	r.mux.RUnlock()
	return ok
}
