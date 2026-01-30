package api

import (
	"encoding/json"
	"net/http"

	"aichatplayers/internal/logging"
	"aichatplayers/internal/planner"
)

type Handler struct {
	Planner *planner.Planner
}

func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	transactionID := RequestIDFromContext(r.Context())
	logging.Infof("request_id=%s transaction_id=%s healthz", transactionID, transactionID)
	respondJSON(w, http.StatusOK, HealthResponse{Status: "ok"})
}

func (h *Handler) Plan(w http.ResponseWriter, r *http.Request) {
	transactionID := RequestIDFromContext(r.Context())
	var req PlanRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		logging.Warnf("request_id=%s transaction_id=%s invalid plan request: %v", transactionID, transactionID, err)
		respondError(w, http.StatusBadRequest, "invalid_json")
		return
	}

	if req.RequestID == "" {
		if transactionID != "" {
			req.RequestID = transactionID
		}
	}
	if transactionID == "" {
		transactionID = req.RequestID
	}

	if payload, err := json.Marshal(req); err == nil {
		logging.Debugf("request_id=%s transaction_id=%s plan_request=%s", req.RequestID, transactionID, string(payload))
	} else {
		logging.Warnf("request_id=%s transaction_id=%s failed to marshal plan request: %v", req.RequestID, transactionID, err)
	}

	response := h.Planner.Plan(req)
	if payload, err := json.Marshal(response); err == nil {
		logging.Debugf("request_id=%s transaction_id=%s plan_response=%s", req.RequestID, transactionID, string(payload))
	} else {
		logging.Warnf("request_id=%s transaction_id=%s failed to marshal plan response: %v", req.RequestID, transactionID, err)
	}
	respondJSON(w, http.StatusOK, response)
}

func (h *Handler) RegisterBots(w http.ResponseWriter, r *http.Request) {
	transactionID := RequestIDFromContext(r.Context())
	var req BotRegisterRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		logging.Warnf("request_id=%s transaction_id=%s invalid register request: %v", transactionID, transactionID, err)
		respondError(w, http.StatusBadRequest, "invalid_json")
		return
	}

	count := h.Planner.RegisterBots(req.ServerID, req.Bots)
	logging.Infof("request_id=%s transaction_id=%s register_bots server_id=%s bots=%d registered=%d", transactionID, transactionID, req.ServerID, len(req.Bots), count)
	respondJSON(w, http.StatusOK, BotRegisterResponse{Registered: count})
}

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(payload); err != nil {
		logging.Warnf("failed to encode response: %v", err)
	}
}

func respondError(w http.ResponseWriter, status int, code string) {
	respondJSON(w, status, map[string]string{"error": code})
}
