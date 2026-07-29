package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/netip"
)

type handler struct {
	svc Service
}

type authRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
	IP       string `json:"ip"`
}

type authResponse struct {
	OK bool `json:"ok"`
}

type resetRequest struct {
	Login string `json:"login"`
	IP    string `json:"ip"`
}

type subnetRequest struct {
	Subnet string `json:"subnet"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func (h *handler) auth(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Login == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "login and password are required")
		return
	}
	if _, err := netip.ParseAddr(req.IP); err != nil {
		writeError(w, http.StatusBadRequest, "invalid ip: "+err.Error())
		return
	}
	ok, err := h.svc.Allow(r.Context(), req.Login, req.Password, req.IP)
	if err != nil {
		writeInternal(w, "auth", err)
		return
	}
	writeJSON(w, http.StatusOK, authResponse{OK: ok})
}

func (h *handler) reset(w http.ResponseWriter, r *http.Request) {
	var req resetRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Login == "" || req.IP == "" {
		writeError(w, http.StatusBadRequest, "login and ip are required")
		return
	}
	if _, err := netip.ParseAddr(req.IP); err != nil {
		writeError(w, http.StatusBadRequest, "invalid ip: "+err.Error())
		return
	}
	if err := h.svc.Reset(r.Context(), req.Login, req.IP); err != nil {
		writeInternal(w, "reset", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) addWhitelist(w http.ResponseWriter, r *http.Request) {
	h.mutateSubnet(w, r, h.svc.AddToWhitelist, "add to whitelist")
}

func (h *handler) removeWhitelist(w http.ResponseWriter, r *http.Request) {
	h.mutateSubnet(w, r, h.svc.RemoveFromWhitelist, "remove from whitelist")
}

func (h *handler) addBlacklist(w http.ResponseWriter, r *http.Request) {
	h.mutateSubnet(w, r, h.svc.AddToBlacklist, "add to blacklist")
}

func (h *handler) removeBlacklist(w http.ResponseWriter, r *http.Request) {
	h.mutateSubnet(w, r, h.svc.RemoveFromBlacklist, "remove from blacklist")
}

func (h *handler) mutateSubnet(
	w http.ResponseWriter,
	r *http.Request,
	fn func(context.Context, netip.Prefix) error,
	op string,
) {
	var req subnetRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	prefix, err := netip.ParsePrefix(req.Subnet)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid subnet: "+err.Error())
		return
	}
	if err := fn(r.Context(), prefix); err != nil {
		writeInternal(w, op, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("api: encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

func writeInternal(w http.ResponseWriter, op string, err error) {
	log.Printf("api: %s: %v", op, err)
	writeError(w, http.StatusInternalServerError, "internal error")
}
