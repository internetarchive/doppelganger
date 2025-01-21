package handlers

import "net/http"

// healthcheck is a simple healthcheck handler, it is primarily used to check if the server is running
// in the context of a Nomad deployment.
func Healthcheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
