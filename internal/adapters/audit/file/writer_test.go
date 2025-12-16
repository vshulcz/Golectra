package file

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/vshulcz/Golectra/internal/services/audit"
)

func TestWriter_Notify_AppendsJSONLine(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/audit.log"
	w := New(path)
	evt := audit.Event{Timestamp: 1, Metrics: []string{"Alloc"}, IPAddress: "127.0.0.1"}
	if err := w.Notify(context.Background(), evt); err != nil {
		t.Fatalf("Notify error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	var decoded audit.Event
	if err := json.Unmarshal(data[:len(data)-1], &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.IPAddress != evt.IPAddress || decoded.Timestamp != evt.Timestamp {
		t.Fatalf("decoded mismatch: %+v", decoded)
	}
}
