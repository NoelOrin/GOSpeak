package bus

import (
	"bufio"
	"encoding/json"
	"os"
	"sync"
)

// walRecord 是 WAL 落盘记录，字段必须导出供 encoding/json 序列化。
type walRecord struct {
	Subject string   `json:"subject"`
	Env     Envelope `json:"env"`
}

// pendingWAL 将断线期间 pending 的事件追加到磁盘 JSONL 文件，
// 每次 Append 后 fsync，进程崩溃后可从 ReadAll 恢复。
type pendingWAL struct {
	mu   sync.Mutex
	path string
	f    *os.File
}

func newPendingWAL(path string) (*pendingWAL, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &pendingWAL{path: path, f: f}, nil
}

func (w *pendingWAL) Append(subject string, env Envelope) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	data, err := json.Marshal(walRecord{Subject: subject, Env: env})
	if err != nil {
		return err
	}
	if _, err := w.f.Write(append(data, '\n')); err != nil {
		return err
	}
	return w.f.Sync()
}

func (w *pendingWAL) ReadAll() ([]pendingEnvelope, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	f, err := os.Open(w.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []pendingEnvelope
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 4*1024*1024)
	for sc.Scan() {
		var rec walRecord
		if err := json.Unmarshal(sc.Bytes(), &rec); err == nil {
			out = append(out, pendingEnvelope{subject: rec.Subject, env: rec.Env})
		}
	}
	return out, sc.Err()
}

func (w *pendingWAL) Truncate() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.f.Truncate(0); err != nil {
		return err
	}
	_, err := w.f.Seek(0, 0)
	return err
}

func (w *pendingWAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.f.Close()
}
