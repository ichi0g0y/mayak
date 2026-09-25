package tracker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSetTaskUsesExpectedPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/progress/task/5a27b87686f77460de0252a8" || r.Header.Get("Authorization") != "Bearer PVE_token" {
			t.Fatalf("unexpected request: %s %s", r.URL.Path, r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()
	client := New()
	client.baseURL = server.URL
	if err := client.SetTask(context.Background(), "PVE_token", "5a27b87686f77460de0252a8", "completed"); err != nil {
		t.Fatal(err)
	}
}

func TestModeForToken(t *testing.T) {
	for token, want := range map[string]Mode{"PVP_abc": ModePVP, "PVE_abc": ModePVE, "SZN_abc": ModeSeasonal} {
		got, ok := ModeForToken(token)
		if !ok || got != want {
			t.Fatalf("%s: got %s %v", token, got, ok)
		}
	}
}

func TestProgressDecodesCurrentResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"tasksProgress":[{"id":"5a27b87686f77460de0252a8","complete":true,"failed":false}],"hideoutModulesProgress":[{"id":"wall-1","complete":true},{"id":"wall-2","complete":false}],"displayName":"PMC","playerLevel":42},"meta":{"gameMode":"seasonal"}}`))
	}))
	defer server.Close()
	client := New()
	client.baseURL = server.URL
	progress, err := client.Progress(context.Background(), "SZN_token")
	if len(progress.Data.HideoutModules) != 2 || !progress.Data.HideoutModules[0].Complete || progress.Data.HideoutModules[1].Complete {
		t.Fatal("hideout modules not decoded")
	}
	if err != nil {
		t.Fatal(err)
	}
	if progress.Meta.GameMode != "seasonal" || progress.Data.DisplayName != "PMC" || len(progress.Data.Tasks) != 1 || !progress.Data.Tasks[0].Complete {
		t.Fatalf("unexpected progress: %+v", progress)
	}
}

func TestHasPermissions(t *testing.T) {
	if !HasPermissions(TokenInfo{Permissions: []string{"GP", "WP"}}, "gp", "wp") {
		t.Fatal("expected permissions")
	}
}
