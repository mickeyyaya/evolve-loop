package lifecycle

import "github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"

const CodeDeliveredOrphan signalcenter.Code = "INBOX_DELIVERED_ORPHAN"

const CodeDeliveryFailed signalcenter.Code = "INBOX_DELIVERY_FAILED"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleInbox, CodeDeliveredOrphan, "an item in delivered/ has no member in any wave record, so no integration step will end its state; the item stays delivered and is never released by itself; fields.id, step=orphan_delivered")
	signalcenter.RegisterCode(signalcenter.ModuleInbox, CodeDeliveryFailed, "the move of a committed item to delivered/ failed after a delivered ship; the item ends in quarantine/ or, when the hold also failed, stays in processing/, so that no lane draws it, and an operator releases it; fields.id, step=deliver, end_state")
}
