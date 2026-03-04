package api

import "net/http"

func (a *API) HandleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if a.auth == nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Authentication is not configured",
			Code:    http.StatusNotFound,
		})
		return
	}
	a.auth.StartLogin(w, r)
}

func (a *API) HandleAuthCallback(w http.ResponseWriter, r *http.Request) {
	if a.auth == nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Authentication is not configured",
			Code:    http.StatusNotFound,
		})
		return
	}
	a.auth.CompleteLogin(w, r)
}

func (a *API) HandleAuthLogoutRedirect(w http.ResponseWriter, r *http.Request) {
	if a.auth == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	a.auth.LogoutRedirect(w, r)
}

func (a *API) HandleAuthMe(w http.ResponseWriter, r *http.Request) {
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "Authentication required",
			Code:    http.StatusUnauthorized,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user": principal,
	})
}
