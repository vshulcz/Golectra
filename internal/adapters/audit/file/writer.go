package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/vshulcz/Golectra/internal/services/audit"
)

// Writer appends audit events to a local newline-delimited JSON file.
type Writer struct {
	path string
	mu   sync.Mutex
}

// New creates a Writer that writes every event to the provided filesystem path.
func New(path string) *Writer {
	return &Writer{path: path}
}

// Notify marshals the audit event and atomically appends it to the writer's file.
func (w *Writer) Notify(_ context.Context, evt audit.Event) (retErr error) {
	if w == nil || w.path == "" {
		return nil
	}

	payload, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal audit event: %w", err)
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open audit file: %w", err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && retErr == nil {
			retErr = fmt.Errorf("close audit file: %w", cerr)
		}
	}()

	if _, err := f.Write(append(payload, '\n')); err != nil {
		return fmt.Errorf("write audit file: %w", err)
	}
	return nil
}
