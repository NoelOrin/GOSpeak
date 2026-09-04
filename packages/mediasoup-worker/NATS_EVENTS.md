# Optional NATS events (Phase 4)

Default control plane remains **HTTP bridge** (`:3012`).

Future enhancement (not required for core multi-instance signaling):

- Set `NATS_URL` / `NATS_SUBJECT_PREFIX` on the worker
- Publish producer/transport close to `{prefix}.mediasoup.event`
- Go backend may subscribe and fanout `sfu:producer-closed` without extra HTTP polling

Until implemented, leave path stays: WebSocket leave → Hub → HTTP `CloseParticipant`.
