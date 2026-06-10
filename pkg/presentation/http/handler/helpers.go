package handler

import "net/http"

func parseNamespace(r *http.Request) string {
	ns := r.URL.Query().Get("ns")
	if ns != "" {
		return ns + "/"
	}

	return ""
}
