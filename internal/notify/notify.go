package notify

// Notifier provides channel-based notification for newly created events/deliveries.
// Channels are capacity-1 for non-blocking semantics: multiple notifications between
// dispatches collapse into a single signal (coalescing), which is correct because
// the dispatcher always does a batch-claim loop after being woken.
type Notifier struct {
	eventCh    chan struct{}
	deliveryCh chan struct{}
}

// NewNotifier creates a new Notifier with capacity-1 channels.
func NewNotifier() *Notifier {
	return &Notifier{
		eventCh:    make(chan struct{}, 1),
		deliveryCh: make(chan struct{}, 1),
	}
}

// NotifyEvent signals that a new event has been inserted.
// Non-blocking: drops the signal if the channel is full (coalescing).
func (n *Notifier) NotifyEvent() {
	if n == nil {
		return
	}
	select {
	case n.eventCh <- struct{}{}:
	default:
	}
}

// NotifyDelivery signals that a new delivery has been created.
// Non-blocking: drops the signal if the channel is full (coalescing).
func (n *Notifier) NotifyDelivery() {
	if n == nil {
		return
	}
	select {
	case n.deliveryCh <- struct{}{}:
	default:
	}
}

// EventCh returns the event notification channel (read-only for consumers).
// Returns nil if the receiver is nil, which causes select to never match
// (the dispatcher falls back to ticker-only polling).
func (n *Notifier) EventCh() <-chan struct{} {
	if n == nil {
		return nil
	}
	return n.eventCh
}

// DeliveryCh returns the delivery notification channel (read-only for consumers).
// Returns nil if the receiver is nil, which causes select to never match.
func (n *Notifier) DeliveryCh() <-chan struct{} {
	if n == nil {
		return nil
	}
	return n.deliveryCh
}
