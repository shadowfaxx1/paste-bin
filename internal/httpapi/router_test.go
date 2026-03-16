package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"personal-api-service/internal/store"
)

type fakeMessageStore struct {
	messages  []store.Message
	pingErr   error
	createErr error
	listErr   error
}

func (f *fakeMessageStore) CreateMessage(_ context.Context, text string) (store.Message, error) {
	if f.createErr != nil {
		return store.Message{}, f.createErr
	}

	message := store.Message{
		ID:        int64(len(f.messages) + 1),
		Text:      text,
		CreatedAt: time.Unix(0, 0).UTC(),
	}
	f.messages = append([]store.Message{message}, f.messages...)
	return message, nil
}

func (f *fakeMessageStore) ListMessages(_ context.Context, limit int) ([]store.Message, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	if limit > len(f.messages) {
		limit = len(f.messages)
	}
	return f.messages[:limit], nil
}

func (f *fakeMessageStore) Ping(_ context.Context) error {
	return f.pingErr
}

func TestCreateAndListMessages(t *testing.T) {
	fakeStore := &fakeMessageStore{}
	handler := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), fakeStore)

	postReq := httptest.NewRequest(http.MethodPost, "/message", bytes.NewBufferString(`{"text":"hello"}`))
	postRec := httptest.NewRecorder()
	handler.ServeHTTP(postRec, postReq)

	if postRec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, postRec.Code)
	}

	var created store.Message
	if err := json.NewDecoder(postRec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.Text != "hello" {
		t.Fatalf("expected text hello, got %q", created.Text)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/message", nil)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, getRec.Code)
	}

	var listed struct {
		Messages []store.Message `json:"messages"`
	}
	if err := json.NewDecoder(getRec.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(listed.Messages))
	}
}

func TestRejectsInvalidLimit(t *testing.T) {
	handler := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeMessageStore{})
	req := httptest.NewRequest(http.MethodGet, "/message?limit=999", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}
