package apiresponse

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

// Envelope is the standard success JSON shape: data + meta (+ optional links).
type Envelope struct {
	Data  interface{} `json:"data"`
	Meta  Meta        `json:"meta"`
	Links *Links      `json:"links,omitempty"`
}

// Meta carries request correlation and optional pagination.
type Meta struct {
	RequestID   string           `json:"request_id"`
	APIVersion  string           `json:"api_version"`
	Pagination  *PaginationMeta  `json:"pagination,omitempty"`
}

// PaginationMeta describes offset-style paging (page/limit).
type PaginationMeta struct {
	Page    int   `json:"page"`
	Limit   int   `json:"limit"`
	Total   int64 `json:"total"`
	HasMore bool  `json:"has_more"`
}

// Links holds HATEOAS-style navigation URLs.
type Links struct {
	Self string `json:"self,omitempty"`
	Next string `json:"next,omitempty"`
	Prev string `json:"prev,omitempty"`
}

const apiVersion = "v1"

// RequestID returns the Chi request ID from context (may be empty).
func RequestID(r *http.Request) string {
	return middleware.GetReqID(r.Context())
}

// WriteJSON writes a success envelope with Content-Type application/json.
func WriteJSON(w http.ResponseWriter, r *http.Request, status int, data interface{}, pagination *PaginationMeta, links *Links) {
	env := Envelope{
		Data: data,
		Meta: Meta{
			RequestID:  RequestID(r),
			APIVersion: apiVersion,
			Pagination: pagination,
		},
		Links: links,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(env)
}
