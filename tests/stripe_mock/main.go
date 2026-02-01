package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type checkoutSession struct {
	ID        string              `json:"id"`
	URL       string              `json:"url"`
	Mode      string              `json:"mode"`
	Customer  string              `json:"customer"`
	Success   string              `json:"success_url"`
	Cancel    string              `json:"cancel_url"`
	LineItems []checkoutLineItem  `json:"line_items"`
	Metadata  map[string]string   `json:"metadata"`
	RawForm   map[string][]string `json:"raw_form"`
}

type checkoutLineItem struct {
	Price    string `json:"price"`
	Quantity int    `json:"quantity"`
}

type serverState struct {
	mu sync.Mutex

	nextCustomer int
	nextSession  int
	nextPortal   int

	lastCustomerForm url.Values
	lastCheckout     *checkoutSession
	lastPortalURL    string
}

func main() {
	// Optional. When empty, we derive it from the incoming request Host header.
	publicBaseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("STRIPE_MOCK_PUBLIC_BASE_URL")), "/")

	state := &serverState{
		nextCustomer: 1,
		nextSession:  1,
		nextPortal:   1,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Stripe API (minimal)
	mux.HandleFunc("/v1/customers", state.handleCreateCustomer)
	mux.HandleFunc("/v1/checkout/sessions", state.handleCreateCheckoutSession(publicBaseURL))
	mux.HandleFunc("/v1/billing_portal/sessions", state.handleCreatePortalSession(publicBaseURL))

	// Test helpers / UI pages (non-Stripe)
	mux.HandleFunc("/test/last-checkout-session", state.handleLastCheckoutSession)
	mux.HandleFunc("/test/checkout/", state.handleCheckoutPage)
	mux.HandleFunc("/test/portal/", state.handlePortalPage)

	addr := ":12111"
	log.Printf("stripe-mock listening on %s", addr)
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func (s *serverState) handleCreateCustomer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id := fmt.Sprintf("cus_test_%d", s.nextCustomer)
	s.nextCustomer++
	s.lastCustomerForm = r.PostForm

	writeJSON(w, http.StatusOK, map[string]any{
		"id": id,
	})
}

func (s *serverState) handleCreateCheckoutSession(publicBaseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}

		session := parseCheckoutSession(r.PostForm)

		baseURL := publicBaseURL
		if baseURL == "" {
			if strings.TrimSpace(r.Host) != "" {
				baseURL = "http://" + r.Host
			} else {
				baseURL = "http://stripe-mock:12111"
			}
		}
		baseURL = strings.TrimRight(baseURL, "/")

		s.mu.Lock()
		session.ID = fmt.Sprintf("cs_test_%d", s.nextSession)
		s.nextSession++
		session.URL = fmt.Sprintf("%s/test/checkout/%s", baseURL, session.ID)
		session.RawForm = copyForm(r.PostForm)
		s.lastCheckout = &session
		s.mu.Unlock()

		writeJSON(w, http.StatusOK, map[string]any{
			"id":  session.ID,
			"url": session.URL,
		})
	}
}

func (s *serverState) handleCreatePortalSession(publicBaseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}

		s.mu.Lock()
		id := fmt.Sprintf("bps_test_%d", s.nextPortal)
		s.nextPortal++
		// Stripe returns a URL to the customer portal; we return a simple page that links back to return_url.
		returnURL := r.PostFormValue("return_url")
		baseURL := publicBaseURL
		if baseURL == "" {
			if strings.TrimSpace(r.Host) != "" {
				baseURL = "http://" + r.Host
			} else {
				baseURL = "http://stripe-mock:12111"
			}
		}
		baseURL = strings.TrimRight(baseURL, "/")

		s.lastPortalURL = fmt.Sprintf("%s/test/portal/%s?return_url=%s", baseURL, id, url.QueryEscape(returnURL))
		outURL := s.lastPortalURL
		s.mu.Unlock()

		writeJSON(w, http.StatusOK, map[string]any{
			"id":  id,
			"url": outURL,
		})
	}
}

func (s *serverState) handleLastCheckoutSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.lastCheckout == nil {
		http.Error(w, "no checkout session created yet", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, s.lastCheckout)
}

func (s *serverState) handleCheckoutPage(w http.ResponseWriter, r *http.Request) {
	// Basic HTML page for Playwright debugging / optional manual flow.
	id := strings.TrimPrefix(r.URL.Path, "/test/checkout/")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	s.mu.Lock()
	session := s.lastCheckout
	s.mu.Unlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<!doctype html>
<html lang="en">
  <head><meta charset="utf-8"><title>Mock Stripe Checkout</title></head>
  <body>
    <h1>Mock Stripe Checkout</h1>
    <p>Session: <code>%s</code></p>
`, htmlEscape(id))

	if session != nil && session.ID == id {
		_, _ = fmt.Fprintf(w, "<p>Mode: <code>%s</code></p>\n", htmlEscape(session.Mode))
		_, _ = fmt.Fprintf(w, "<p>Customer: <code>%s</code></p>\n", htmlEscape(session.Customer))
		if len(session.LineItems) > 0 {
			_, _ = fmt.Fprintf(w, "<h2>Line items</h2><ul>\n")
			for _, li := range session.LineItems {
				_, _ = fmt.Fprintf(w, "<li><code>%s</code> x %d</li>\n", htmlEscape(li.Price), li.Quantity)
			}
			_, _ = fmt.Fprintf(w, "</ul>\n")
		}
		if len(session.Metadata) > 0 {
			keys := make([]string, 0, len(session.Metadata))
			for k := range session.Metadata {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			_, _ = fmt.Fprintf(w, "<h2>Metadata</h2><ul>\n")
			for _, k := range keys {
				_, _ = fmt.Fprintf(w, "<li><code>%s</code>: <code>%s</code></li>\n", htmlEscape(k), htmlEscape(session.Metadata[k]))
			}
			_, _ = fmt.Fprintf(w, "</ul>\n")
		}
		if session.Success != "" || session.Cancel != "" {
			_, _ = fmt.Fprintf(w, "<h2>Return</h2>\n")
			if session.Success != "" {
				_, _ = fmt.Fprintf(w, `<p><a href="%s">Simulate success redirect</a></p>`, htmlEscape(session.Success))
			}
			if session.Cancel != "" {
				_, _ = fmt.Fprintf(w, `<p><a href="%s">Simulate cancel redirect</a></p>`, htmlEscape(session.Cancel))
			}
		}
	}

	_, _ = fmt.Fprintf(w, `
  </body>
</html>`)
}

func (s *serverState) handlePortalPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	returnURL := r.URL.Query().Get("return_url")
	_, _ = fmt.Fprintf(w, `<!doctype html>
<html lang="en">
  <head><meta charset="utf-8"><title>Mock Stripe Portal</title></head>
  <body>
    <h1>Mock Stripe Customer Portal</h1>
    %s
  </body>
</html>`, portalReturnHTML(returnURL))
}

func portalReturnHTML(returnURL string) string {
	if strings.TrimSpace(returnURL) == "" {
		return "<p>No return_url provided.</p>"
	}
	return fmt.Sprintf(`<p><a href="%s">Return to app</a></p>`, htmlEscape(returnURL))
}

func parseCheckoutSession(form url.Values) checkoutSession {
	mode := strings.TrimSpace(form.Get("mode"))
	customer := strings.TrimSpace(form.Get("customer"))
	success := strings.TrimSpace(form.Get("success_url"))
	cancel := strings.TrimSpace(form.Get("cancel_url"))

	items := parseLineItems(form)
	metadata := parseMetadata(form)

	return checkoutSession{
		Mode:      mode,
		Customer:  customer,
		Success:   success,
		Cancel:    cancel,
		LineItems: items,
		Metadata:  metadata,
	}
}

var (
	lineItemPriceRe = regexp.MustCompile(`^line_items\[(\d+)\]\[price\]$`)
	lineItemQtyRe   = regexp.MustCompile(`^line_items\[(\d+)\]\[quantity\]$`)
	metadataRe      = regexp.MustCompile(`^metadata\[(.+)\]$`)
)

func parseLineItems(form url.Values) []checkoutLineItem {
	type tmp struct {
		price string
		qty   int
	}
	items := map[int]*tmp{}

	for k, v := range form {
		if len(v) == 0 {
			continue
		}
		if m := lineItemPriceRe.FindStringSubmatch(k); m != nil {
			i, _ := strconv.Atoi(m[1])
			if items[i] == nil {
				items[i] = &tmp{qty: 1}
			}
			items[i].price = strings.TrimSpace(v[0])
			continue
		}
		if m := lineItemQtyRe.FindStringSubmatch(k); m != nil {
			i, _ := strconv.Atoi(m[1])
			if items[i] == nil {
				items[i] = &tmp{qty: 1}
			}
			qty, err := strconv.Atoi(strings.TrimSpace(v[0]))
			if err == nil && qty > 0 {
				items[i].qty = qty
			}
		}
	}

	indexes := make([]int, 0, len(items))
	for i := range items {
		indexes = append(indexes, i)
	}
	sort.Ints(indexes)

	out := make([]checkoutLineItem, 0, len(indexes))
	for _, i := range indexes {
		it := items[i]
		if it == nil || strings.TrimSpace(it.price) == "" {
			continue
		}
		out = append(out, checkoutLineItem{Price: it.price, Quantity: it.qty})
	}
	return out
}

func parseMetadata(form url.Values) map[string]string {
	out := map[string]string{}
	for k, v := range form {
		if len(v) == 0 {
			continue
		}
		m := metadataRe.FindStringSubmatch(k)
		if m == nil {
			continue
		}
		key := strings.TrimSpace(m[1])
		if key == "" {
			continue
		}
		out[key] = strings.TrimSpace(v[0])
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func copyForm(form url.Values) map[string][]string {
	out := make(map[string][]string, len(form))
	for k, v := range form {
		cp := make([]string, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(v)
}

func htmlEscape(s string) string {
	replacer := strings.NewReplacer(
		`&`, "&amp;",
		`<`, "&lt;",
		`>`, "&gt;",
		`"`, "&quot;",
		`'`, "&#39;",
	)
	return replacer.Replace(s)
}
