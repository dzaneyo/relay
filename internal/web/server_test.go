package web_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/dzaneyo/relay/internal/app"
	"github.com/dzaneyo/relay/internal/model"
	"github.com/dzaneyo/relay/internal/storage"
	"github.com/dzaneyo/relay/internal/web"
)

func testServer(t *testing.T) (*app.App, http.Handler) {
	t.Helper()
	db, err := storage.OpenPath(filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	a := app.New(db)
	return a, web.NewServer(a).Handler()
}

func TestHealthAndNoteAPI(t *testing.T) {
	_, h := testServer(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("health status=%d body=%s", w.Code, w.Body.String())
	}
	body := []byte(`{"name":"Runbook","alias":"ignored","category":"NOTE","notes":"Deploy carefully","favorite":true}`)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/records", bytes.NewReader(body)))
	if w.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", w.Code, w.Body.String())
	}
	var created struct {
		Record struct {
			ID    string `json:"id"`
			Alias string `json:"alias"`
		} `json:"record"`
		Credential any `json:"credential"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Record.Alias != "" || created.Credential != nil {
		t.Fatalf("NOTE returned alias or credential: %s", w.Body.String())
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/records/"+created.Record.ID, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("detail status=%d body=%s", w.Code, w.Body.String())
	}
	updateBody := []byte(`{"name":"Runbook updated","alias":"still-ignored","category":"NOTE","notes":"Updated"}`)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/api/records/"+created.Record.ID, bytes.NewReader(updateBody)))
	if w.Code != http.StatusOK {
		t.Fatalf("NOTE update status=%d body=%s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/records?q=updated", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestEmbeddedFrontendIsServed(t *testing.T) {
	_, h := testServer(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("frontend status=%d body=%s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("relay · Local connection manager")) {
		t.Fatalf("built frontend index was not served: %s", w.Body.String())
	}
}

func TestPutUnknownReturnsNotFound(t *testing.T) {
	a, h := testServer(t)
	recordBody := []byte(`{"name":"Missing","category":"NOTE","notes":"none"}`)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/api/records/missing", bytes.NewReader(recordBody)))
	if w.Code != http.StatusNotFound {
		t.Fatalf("record PUT status=%d body=%s", w.Code, w.Body.String())
	}
	host, err := a.RecordService.Create(context.Background(), model.RecordInput{Name: "Hop", Alias: "hop", Category: model.CategoryHost, SSH: &model.SSHConnection{Host: "10.0.0.1"}})
	if err != nil {
		t.Fatal(err)
	}
	routeBody := []byte(`{"name":"Missing Route","hops":[{"seq":1,"hostRecordId":"` + host.Record.ID + `"}]}`)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/api/routes/missing", bytes.NewReader(routeBody)))
	if w.Code != http.StatusNotFound {
		t.Fatalf("route PUT status=%d body=%s", w.Code, w.Body.String())
	}
}
