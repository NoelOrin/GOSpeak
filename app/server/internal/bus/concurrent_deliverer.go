package bus

// ConcurrentDeliverer fans out broadcasts to a goroutine pool so
// high-fanout Socket.IO rooms do not block the event dispatch path.
type ConcurrentDeliverer struct {
	server SocketServer
	sem    chan struct{}
}

// NewConcurrentDeliverer creates a deliverer with max concurrent writes.
// concurrency=0 defaults to 128.
func NewConcurrentDeliverer(server SocketServer, concurrency int) *ConcurrentDeliverer {
	if concurrency <= 0 {
		concurrency = 128
	}
	return &ConcurrentDeliverer{
		server: server,
		sem:    make(chan struct{}, concurrency),
	}
}

func (d *ConcurrentDeliverer) BroadcastToNamespace(event string, data interface{}) {
	if d == nil || d.server == nil {
		return
	}
	select {
	case d.sem <- struct{}{}:
		go func() {
			defer func() { <-d.sem }()
			d.server.BroadcastToNamespace("/", event, data)
		}()
	default:
		// pool full — fall back to synchronous write
		d.server.BroadcastToNamespace("/", event, data)
	}
}

func (d *ConcurrentDeliverer) BroadcastToRoom(room, event string, data interface{}) {
	if d == nil || d.server == nil {
		return
	}
	select {
	case d.sem <- struct{}{}:
		go func() {
			defer func() { <-d.sem }()
			d.server.BroadcastToRoom("/", room, event, data)
		}()
	default:
		// pool full — fall back to synchronous write (backpressure)
		d.server.BroadcastToRoom("/", room, event, data)
	}
}

// Close depletes the goroutine semaphore to prevent further submits.
func (d *ConcurrentDeliverer) Close() {
	if d == nil || d.sem == nil {
		return
	}
	close(d.sem)
}
