package slack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSend(t *testing.T) {
	var got map[string]string
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		auth = r.Header.Get("Authorization")
		if r.URL.Path == "/chat.postMessage" {
			w.Write([]byte(`{"ok":true}`))
		}
	}))
	defer srv.Close()
	ctx := context.Background()
	if err := Send(ctx, srv.URL+"/hook", "", "", "", Message("Deploy", "api is live")); err != nil || got["text"] != "*Deploy*\napi is live" {
		t.Fatalf("webhook %v %v", got, err)
	}
	if err := Send(ctx, "", "xoxb", srv.URL, "#ops", "hi"); err != nil || got["channel"] != "#ops" || auth != "Bearer xoxb" {
		t.Fatalf("token %v %q %v", got, auth, err)
	}
	if err := Send(ctx, "", "", srv.URL, "#ops", "hi"); err == nil {
		t.Fatal("sent without credentials")
	}
}
