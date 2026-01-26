package api

import (
	"encoding/json"
	"log"
	"net/http"

	"aichatplayers/internal/planner"
)

type Handler struct {
	Planner *planner.Planner
}

func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, HealthResponse{Status: "ok"})
}

func (h *Handler) Plan(w http.ResponseWriter, r *http.Request) {
	var req PlanRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_json")
		return
	}

	if req.RequestID == "" {
		if ctxID := RequestIDFromContext(r.Context()); ctxID != "" {
			req.RequestID = ctxID
		}
	}

	response := h.Planner.Plan(req)
	respondJSON(w, http.StatusOK, response)
}

func (h *Handler) RegisterBots(w http.ResponseWriter, r *http.Request) {
	var req BotRegisterRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_json")
		return
	}

	count := h.Planner.RegisterBots(req.ServerID, req.Bots)
	respondJSON(w, http.StatusOK, BotRegisterResponse{Registered: count})
}

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(payload); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}

func respondError(w http.ResponseWriter, status int, code string) {
	respondJSON(w, status, map[string]string{"error": code})
}
