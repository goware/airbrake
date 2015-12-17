package airbrake

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/context"
)

var (
	sensitiveQueryFields = []string{
		"password",
		"passwd",
		"pass",
		"passphrase",
		"session",
		"token",
		"secret",
		"jwt",
	}
	sensitiveHeaders = []string{
		"Authorization",
		"X-Csrf-Token",
		"Cookie",
	}
	sensitiveCookies = []string{
		"session",
		"auth",
		"jwt",
	}
)

func Recoverer(c *Client, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if c != nil {
			defer c.HTTPPanic(r, w)
		}
		next(w, r)
	}
}

func (c *Client) HTTPPanic(r *http.Request, w http.ResponseWriter) {
	if rval := recover(); rval != nil {
		//debug.PrintStack()

		e := NewEvent("panic")
		e.Stacktrace = NewStacktrace(2, 3, nil)
		e.Context["HTTP"] = NewHttp(r)

		c.send(e)
	}
}

func (c *Client) HTTPPanicWithCtx(r *http.Request, w http.ResponseWriter, ctx context.Context) {
	if rval := recover(); rval != nil {

		e := NewEvent("panic")
		e.Stacktrace = NewStacktrace(2, 3, nil)
		e.Context["http.request"] = NewHttp(r)
		e.Context["http.ctx"] = ctx

		c.send(e)

		http.Error(w, http.StatusText(500), 500)
	}
}

func NewHttp(req *http.Request) *Http {
	proto := "http"
	if req.TLS != nil || req.Header.Get("X-Forwarded-Proto") == "https" {
		proto = "https"
	}
	h := &Http{
		Method:    req.Method,
		UserAgent: req.UserAgent(),
		Cookies:   sanitizeCookies(req.Cookies()),
		Query:     sanitizeQuery(req.URL.Query()).Encode(),
		URL:       proto + "://" + req.Host + req.URL.Path,
		Headers:   sanitizeHeaders(req.Header),
	}
	if addr, port, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		h.Env = map[string]string{"REMOTE_ADDR": addr, "REMOTE_PORT": port}
	}

	return h
}

func sanitizeQuery(query url.Values) url.Values {
	for _, keyword := range sensitiveQueryFields {
		for field := range query {
			if strings.Contains(field, keyword) {
				query[field] = []string{"[redacted]"}
			}
		}
	}
	return query
}

func sanitizeHeaders(headers http.Header) map[string]string {
	h := make(map[string]string, len(headers))
	for k, v := range headers {
		h[k] = strings.Join(v, ",")
		for _, sec := range sensitiveHeaders {
			if k == sec {
				h[k] = "[redacted]"
			}
		}
	}
	return h
}

func sanitizeCookies(cookies []*http.Cookie) []*http.Cookie {
	for i, _ := range cookies {
		for _, s := range sensitiveCookies {
			if cookies[i].Name == s {
				cookies[i].Value = "[redacted]"
			}
		}
	}
	return cookies
}

type Http struct {
	URL       string
	Method    string
	Query     string
	UserAgent string
	Cookies   []*http.Cookie
	Headers   map[string]string
	Env       map[string]string

	// Must be either a string or map[string]string
	Data interface{}
}
