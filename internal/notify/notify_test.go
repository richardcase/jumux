package notify

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/richardcase/jumux/internal/run"
)

func TestSend(t *testing.T) {
	tests := []struct {
		name     string
		goos     string
		wantCmd  string // "" means no command should run
		wantArgs []string
	}{
		{
			name:     "darwin uses osascript",
			goos:     "darwin",
			wantCmd:  "osascript",
			wantArgs: []string{"-e", `display notification "hi there" with title "jumux: auth"`},
		},
		{
			name:     "linux uses notify-send",
			goos:     "linux",
			wantCmd:  "notify-send",
			wantArgs: []string{"jumux: auth", "hi there"},
		},
		{
			name: "other platforms are a no-op",
			goos: "windows",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fr := &run.FakeRunner{}
			err := send(fr, tt.goos, "jumux: auth", "hi there")
			if err != nil {
				t.Fatalf("send() error = %v", err)
			}
			if tt.wantCmd == "" {
				if len(fr.Calls) != 0 {
					t.Fatalf("expected no command, ran:\n%s", fr.CommandLines())
				}
				return
			}
			if len(fr.Calls) != 1 {
				t.Fatalf("expected 1 command, ran:\n%s", fr.CommandLines())
			}
			got := fr.Calls[0]
			if got.Name != tt.wantCmd {
				t.Errorf("command = %q, want %q", got.Name, tt.wantCmd)
			}
			if len(got.Args) != len(tt.wantArgs) {
				t.Fatalf("args = %v, want %v", got.Args, tt.wantArgs)
			}
			for i, a := range tt.wantArgs {
				if got.Args[i] != a {
					t.Errorf("args[%d] = %q, want %q", i, got.Args[i], a)
				}
			}
		})
	}
}

func TestSendWebhook(t *testing.T) {
	var got WebhookPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	if err := SendWebhook(server.Client(), server.URL, "jumux: auth", "waiting"); err != nil {
		t.Fatal(err)
	}
	if got.Title != "jumux: auth" || got.Message != "waiting" {
		t.Errorf("unexpected payload: %+v", got)
	}
}

func TestSendWebhookErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	if err := SendWebhook(server.Client(), server.URL, "title", "msg"); err == nil {
		t.Fatal("expected an error for a 500 response")
	}
}

func TestSendWebhookConnectionError(t *testing.T) {
	if err := SendWebhook(nil, "http://127.0.0.1:0", "title", "msg"); err == nil {
		t.Fatal("expected a connection error")
	}
}

func TestSendPropagatesRunnerError(t *testing.T) {
	wantErr := errors.New("boom")
	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		return "", wantErr
	}}
	if err := send(fr, "darwin", "title", "msg"); !errors.Is(err, wantErr) {
		t.Fatalf("send() error = %v, want %v", err, wantErr)
	}
}
