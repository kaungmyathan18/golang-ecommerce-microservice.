package apiresponse

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
)

// Problem follows RFC 7807 Problem Details (application/problem+json).
type Problem struct {
	Type       string         `json:"type"`
	Title      string         `json:"title"`
	Status     int            `json:"status"`
	Detail     string         `json:"detail,omitempty"`
	Instance   string         `json:"instance"`
	RequestID  string         `json:"request_id,omitempty"`
	Errors     []FieldProblem `json:"errors,omitempty"`
}

// FieldProblem is an extension field for validation issues.
type FieldProblem struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// WriteProblem sends RFC 7807 JSON with Content-Type application/problem+json.
func WriteProblem(w http.ResponseWriter, r *http.Request, status int, typ, title, detail string, errs []FieldProblem) {
	p := Problem{
		Type:      typ,
		Title:     title,
		Status:    status,
		Detail:    detail,
		Instance:  instancePath(r),
		RequestID: RequestID(r),
		Errors:    errs,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(p)
}

// ProblemTypeURI builds a stable type URI for this request (scheme + host + /problems/<name>).
func ProblemTypeURI(r *http.Request, name string) string {
	return origin(r) + "/problems/" + name
}

func origin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if xfp := r.Header.Get("X-Forwarded-Proto"); xfp != "" {
		scheme = xfp
	}
	return scheme + "://" + r.Host
}

func instancePath(r *http.Request) string {
	if r.URL != nil {
		return r.URL.Path
	}
	return "/"
}

// PageLinks builds self/next/prev URLs for paginated list responses.
func PageLinks(r *http.Request, page, limit int, hasMore bool) *Links {
	base := *r.URL
	q := base.Query()
	q.Set("page", strconv.Itoa(page))
	q.Set("limit", strconv.Itoa(limit))
	base.RawQuery = q.Encode()
	self := absoluteURL(r, &base)

	var nextURL, prevURL string
	if hasMore {
		u := *r.URL
		q2 := u.Query()
		q2.Set("page", strconv.Itoa(page+1))
		q2.Set("limit", strconv.Itoa(limit))
		u.RawQuery = q2.Encode()
		nextURL = absoluteURL(r, &u)
	}
	if page > 1 {
		u := *r.URL
		q2 := u.Query()
		q2.Set("page", strconv.Itoa(page-1))
		q2.Set("limit", strconv.Itoa(limit))
		u.RawQuery = q2.Encode()
		prevURL = absoluteURL(r, &u)
	}
	return &Links{
		Self: self,
		Next: nextURL,
		Prev: prevURL,
	}
}

func absoluteURL(r *http.Request, u *url.URL) string {
	u2 := *u
	if u2.Host == "" {
		u2.Host = r.Host
	}
	if u2.Scheme == "" {
		u2.Scheme = "http"
		if r.TLS != nil {
			u2.Scheme = "https"
		}
		if xfp := r.Header.Get("X-Forwarded-Proto"); xfp != "" {
			u2.Scheme = xfp
		}
	}
	return u2.String()
}

// WriteInternalError sends a generic 500 problem without leaking err to clients.
func WriteInternalError(w http.ResponseWriter, r *http.Request, logReason error) {
	_ = logReason
	WriteProblem(w, r, http.StatusInternalServerError,
		ProblemTypeURI(r, "internal-error"),
		"Internal Server Error",
		"An unexpected error occurred.",
		nil,
	)
}
