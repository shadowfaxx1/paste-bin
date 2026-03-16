package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"personal-api-service/internal/store"
)

type MessageStore interface {
	CreateMessage(ctx context.Context, text string) (store.Message, error)
	ListMessages(ctx context.Context, limit int) ([]store.Message, error)
	Ping(ctx context.Context) error
}

type KVStore interface {
	SetKV(ctx context.Context, key, value string) (store.KVEntry, error)
	GetKV(ctx context.Context, key string) (store.KVEntry, error)
}

type router struct {
	logger       *slog.Logger
	messageStore MessageStore
	kvStore      KVStore
}

type createMessageRequest struct {
	Text string `json:"text"`
}

type setKVRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func NewRouter(logger *slog.Logger, messageStore MessageStore, kvStore KVStore) http.Handler {
	r := &router{
		logger:       logger,
		messageStore: messageStore,
		kvStore:      kvStore,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", r.handleRoot)
	mux.HandleFunc("GET /healthz", r.handleHealth)
	mux.HandleFunc("GET /message", r.handleListMessages)
	mux.HandleFunc("POST /message", r.handleCreateMessage)
	mux.HandleFunc("POST /kv", r.handleSetKV)
	mux.HandleFunc("GET /kv/{key}", r.handleGetKV)

	return r.withMiddleware(mux)
}

func (r *router) withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		startedAt := time.Now()
		w.Header().Set("Content-Type", "application/json")

		wrapped := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, req)

		r.logger.Info("request completed",
			"method", req.Method,
			"path", req.URL.Path,
			"status", wrapped.statusCode,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"remote_addr", req.RemoteAddr,
		)
	})
}

func (r *router) handleRoot(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"service": "personal-api-service",
		"status":  "ok",
	})
}

func (r *router) handleHealth(w http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
	defer cancel()

	if err := r.messageStore.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "database unavailable"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *router) handleCreateMessage(w http.ResponseWriter, req *http.Request) {
	req.Body = http.MaxBytesReader(w, req.Body, 1<<20)
	defer req.Body.Close()

	var payload createMessageRequest
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "request body must contain a single JSON object"})
		return
	}

	text := strings.TrimSpace(payload.Text)
	if text == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "text is required"})
		return
	}

	ctx, cancel := context.WithTimeout(req.Context(), 3*time.Second)
	defer cancel()

	message, err := r.messageStore.CreateMessage(ctx, text)
	if err != nil {
		r.logger.Error("create message failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to create message"})
		return
	}

	writeJSON(w, http.StatusCreated, message)
}

func (r *router) handleListMessages(w http.ResponseWriter, req *http.Request) {
	limit := 50
	if rawLimit := strings.TrimSpace(req.URL.Query().Get("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 || parsed > 200 {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "limit must be between 1 and 200"})
			return
		}
		limit = parsed
	}

	ctx, cancel := context.WithTimeout(req.Context(), 3*time.Second)
	defer cancel()

	messages, err := r.messageStore.ListMessages(ctx, limit)
	if err != nil {
		r.logger.Error("list messages failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to list messages"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"messages": messages})
}

func (r *router) handleSetKV(w http.ResponseWriter, req *http.Request) {
	req.Body = http.MaxBytesReader(w, req.Body, 1<<20)
	defer req.Body.Close()

	var payload setKVRequest
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	payload.Key = strings.TrimSpace(payload.Key)
	payload.Value = strings.TrimSpace(payload.Value)

	if payload.Key == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "key is required"})
		return
	}
	if payload.Value == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "value is required"})
		return
	}

	ctx, cancel := context.WithTimeout(req.Context(), 3*time.Second)
	defer cancel()

	entry, err := r.kvStore.SetKV(ctx, payload.Key, payload.Value)
	if err != nil {
		r.logger.Error("set kv failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to set key"})
		return
	}

	writeJSON(w, http.StatusOK, entry)
}

func (r *router) handleGetKV(w http.ResponseWriter, req *http.Request) {
	key := req.PathValue("key")
	if key == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "key is required"})
		return
	}

	ctx, cancel := context.WithTimeout(req.Context(), 3*time.Second)
	defer cancel()

	entry, err := r.kvStore.GetKV(ctx, key)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "key not found"})
			return
		}
		r.logger.Error("get kv failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to get key"})
		return
	}

	writeJSON(w, http.StatusOK, entry)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}
