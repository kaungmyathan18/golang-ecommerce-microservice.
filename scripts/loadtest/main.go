package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type config struct {
	baseURL   string
	workers   int
	duration  time.Duration
	scenario  string
	thinkMS   int
	catalogN  int
}

type setupData struct {
	token      string
	userID     string
	productIDs []string
}

type result struct {
	status  int
	elapsed time.Duration
	step    string
}

type stats struct {
	total   int64
	ok      int64
	r429    int64
	other   int64
	latency []time.Duration
	byStep  map[string]*stepStats
}

type stepStats struct {
	total int64
	ok    int64
	err   int64
}

type journeyStats struct {
	started   int64
	completed int64
	failed    int64
}

func (s *stats) accepted() int64 { return s.ok + s.r429 }

func main() {
	rand.Seed(time.Now().UnixNano())
	cfg := parseConfig()
	fmt.Printf("=== Load test ===\n")
	fmt.Printf("Target:    %s\n", cfg.baseURL)
	fmt.Printf("Scenario:  %s\n", cfg.scenario)
	fmt.Printf("Workers:   %d\n", cfg.workers)
	fmt.Printf("Duration:  %s\n", cfg.duration)
	if cfg.scenario == "realistic" {
		fmt.Printf("Think:     %dms between steps\n", cfg.thinkMS)
		fmt.Printf("Catalog:   %d products\n", cfg.catalogN)
	}
	fmt.Println()

	client := &http.Client{Timeout: 30 * time.Second}
	if err := waitForGateway(client, cfg.baseURL); err != nil {
		fmt.Fprintf(os.Stderr, "gateway: %v\n", err)
		os.Exit(1)
	}

	switch cfg.scenario {
	case "realistic":
		runRealistic(client, cfg)
	default:
		runSimple(client, cfg)
	}
}

func runSimple(client *http.Client, cfg config) {
	data, err := setupCatalog(client, cfg.baseURL, 1)
	if err != nil {
		fmt.Fprintf(os.Stderr, "setup: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Setup OK (user=%s product=%s)\n\n", data.userID, data.productIDs[0])

	runFn, err := simpleScenario(cfg.scenario, cfg.baseURL, data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scenario: %v\n", err)
		os.Exit(1)
	}

	s := runLoad(client, cfg.workers, cfg.duration, runFn)
	printReport(s, cfg.duration)
	if s.total == 0 || s.other > 0 {
		os.Exit(1)
	}
}

func runRealistic(client *http.Client, cfg config) {
	data, err := setupCatalog(client, cfg.baseURL, cfg.catalogN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "setup: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Catalog ready (%d products, stock=50000 each)\n\n", len(data.productIDs))

	s := &stats{byStep: make(map[string]*stepStats)}
	js := &journeyStats{}
	stop := time.Now().Add(cfg.duration)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var userSeq atomic.Uint64

	for range cfg.workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(stop) {
				atomic.AddInt64(&js.started, 1)
				results, ok := userJourney(client, cfg.baseURL, data.productIDs, cfg.thinkMS, userSeq.Add(1))
				mu.Lock()
				for _, r := range results {
					s.record(r)
				}
				if ok {
					atomic.AddInt64(&js.completed, 1)
				} else {
					atomic.AddInt64(&js.failed, 1)
				}
				mu.Unlock()
				think(cfg.thinkMS * 2)
			}
		}()
	}
	wg.Wait()

	printReport(s, cfg.duration)
	printJourneyReport(js)
	if s.total == 0 || js.completed == 0 {
		if js.completed == 0 {
			fmt.Fprintln(os.Stderr, "\nNo user journeys completed — raise RATE_LIMIT_RPM or reduce LOAD_WORKERS.")
		}
		os.Exit(1)
	}
	if pct(s.other, s.total) > 15 {
		fmt.Fprintf(os.Stderr, "\nToo many hard errors (%.1f%% > 15%%).\n", pct(s.other, s.total))
		os.Exit(1)
	}
}

func parseConfig() config {
	baseURL := env("GATEWAY_URL", "http://localhost:8080")
	workers := envInt("LOAD_WORKERS", 10)
	durationSec := envInt("LOAD_DURATION_SEC", 10)
	scenario := env("LOAD_SCENARIO", "products")
	thinkMS := envInt("LOAD_THINK_MS", 200)
	catalogN := envInt("LOAD_CATALOG_PRODUCTS", 5)

	fs := flag.NewFlagSet("loadtest", flag.ExitOnError)
	fs.StringVar(&baseURL, "url", baseURL, "gateway base URL")
	fs.IntVar(&workers, "workers", workers, "concurrent workers")
	var duration time.Duration
	fs.DurationVar(&duration, "duration", time.Duration(durationSec)*time.Second, "test duration")
	fs.StringVar(&scenario, "scenario", scenario, "health|products|orders|mixed|realistic")
	fs.IntVar(&thinkMS, "think-ms", thinkMS, "think time between steps (realistic scenario)")
	fs.IntVar(&catalogN, "catalog", catalogN, "products seeded in catalog (realistic scenario)")
	_ = fs.Parse(os.Args[1:])

	if workers < 1 {
		workers = 1
	}
	if duration <= 0 {
		duration = 10 * time.Second
	}
	if catalogN < 1 {
		catalogN = 1
	}
	return config{
		baseURL:  strings.TrimRight(baseURL, "/"),
		workers:  workers,
		duration: duration,
		scenario: strings.ToLower(scenario),
		thinkMS:  thinkMS,
		catalogN: catalogN,
	}
}

func waitForGateway(client *http.Client, baseURL string) error {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		res, err := client.Get(baseURL + "/health/live")
		if err == nil {
			res.Body.Close()
			if res.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("gateway not reachable at %s", baseURL)
}

func setupCatalog(client *http.Client, baseURL string, productCount int) (*setupData, error) {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	email := fmt.Sprintf("load-admin-%s@example.com", suffix)
	password := "password123"

	if _, err := postJSON(client, baseURL+"/api/v1/auth/register", map[string]string{
		"email": email, "name": "Load Admin", "password": password,
	}, ""); err != nil {
		return nil, fmt.Errorf("register admin: %w", err)
	}

	loginBody, err := postJSON(client, baseURL+"/api/v1/auth/login", map[string]string{
		"email": email, "password": password,
	}, "")
	if err != nil {
		return nil, fmt.Errorf("login admin: %w", err)
	}
	token, _ := loginBody["access_token"].(string)
	user, _ := loginBody["user"].(map[string]any)
	userID, _ := user["id"].(string)
	if token == "" || userID == "" {
		return nil, fmt.Errorf("login response missing token or user id")
	}

	catBody, err := postJSON(client, baseURL+"/api/v1/categories", map[string]string{
		"name": "Load-Category-" + suffix,
	}, token)
	if err != nil {
		return nil, fmt.Errorf("create category: %w", err)
	}
	categoryID, _ := catBody["id"].(string)

	productIDs := make([]string, 0, productCount)
	for i := range productCount {
		prodBody, err := postJSON(client, baseURL+"/api/v1/products", map[string]any{
			"name": fmt.Sprintf("Load-Product-%s-%d", suffix, i+1),
			"description": "load test catalog item",
			"price":       9.99 + float64(i),
			"category_id": categoryID,
			"stock":       50000,
		}, token)
		if err != nil {
			return nil, fmt.Errorf("create product %d: %w", i+1, err)
		}
		productID, _ := prodBody["id"].(string)
		if productID == "" {
			return nil, fmt.Errorf("create product %d: missing id", i+1)
		}
		productIDs = append(productIDs, productID)
	}

	return &setupData{token: token, userID: userID, productIDs: productIDs}, nil
}

// userJourney simulates one shopper: register → login → browse → order → check order.
func userJourney(client *http.Client, baseURL string, productIDs []string, thinkMS int, seq uint64) ([]result, bool) {
	var out []result
	record := func(step, method, url string, body any, token string) (int, bool) {
		r := doRequest(client, method, url, body, token)
		r.step = step
		out = append(out, r)
		ok := r.status >= 200 && r.status < 300
		return r.status, ok
	}

	email := fmt.Sprintf("shopper-%d-%d@example.com", time.Now().UnixNano(), seq)
	password := "password123"
	name := fmt.Sprintf("Shopper %d", seq)

	_, ok := record("register", http.MethodPost, baseURL+"/api/v1/auth/register", map[string]string{
		"email": email, "name": name, "password": password,
	}, "")
	if !ok {
		return out, false
	}
	think(thinkMS)

	loginData, err := postJSON(client, baseURL+"/api/v1/auth/login", map[string]string{
		"email": email, "password": password,
	}, "")
	if err != nil {
		out = append(out, result{status: 0, step: "login"})
		return out, false
	}
	out = append(out, result{status: http.StatusOK, step: "login"})
	token, _ := loginData["access_token"].(string)
	user, _ := loginData["user"].(map[string]any)
	userID, _ := user["id"].(string)
	if token == "" || userID == "" {
		out = append(out, result{status: 0, step: "login"})
		return out, false
	}
	think(thinkMS)

	_, ok = record("browse", http.MethodGet, baseURL+"/api/v1/products?page=1&limit=20", nil, token)
	if !ok {
		return out, false
	}
	think(thinkMS)

	productID := productIDs[rand.Intn(len(productIDs))]
	qty := 1 + rand.Intn(2)

	status, ok := record("order", http.MethodPost, baseURL+"/api/v1/orders", map[string]any{
		"user_id": userID, "product_id": productID, "quantity": qty,
	}, token)
	if !ok {
		if status == http.StatusConflict {
			// insufficient stock under heavy load — count as partial success
			return out, true
		}
		return out, false
	}
	think(thinkMS)

	_, ok = record("profile", http.MethodGet, baseURL+"/api/v1/auth/me", nil, token)
	return out, ok
}

func simpleScenario(name, baseURL string, data *setupData) (func(*http.Client) result, error) {
	productID := data.productIDs[0]
	switch name {
	case "health":
		return func(c *http.Client) result {
			return doRequest(c, http.MethodGet, baseURL+"/health/live", nil, "")
		}, nil
	case "products":
		return func(c *http.Client) result {
			r := doRequest(c, http.MethodGet, baseURL+"/api/v1/products", nil, data.token)
			r.step = "products"
			return r
		}, nil
	case "orders":
		body := map[string]any{"user_id": data.userID, "product_id": productID, "quantity": 1}
		return func(c *http.Client) result {
			r := doRequest(c, http.MethodPost, baseURL+"/api/v1/orders", body, data.token)
			r.step = "order"
			return r
		}, nil
	case "mixed":
		i := uint64(0)
		return func(c *http.Client) result {
			n := atomic.AddUint64(&i, 1)
			switch n % 3 {
			case 0:
				r := doRequest(c, http.MethodGet, baseURL+"/api/v1/products", nil, data.token)
				r.step = "products"
				return r
			case 1:
				r := doRequest(c, http.MethodGet, baseURL+"/api/v1/products?page=1&limit=10", nil, data.token)
				r.step = "products"
				return r
			default:
				body := map[string]any{"user_id": data.userID, "product_id": productID, "quantity": 1}
				r := doRequest(c, http.MethodPost, baseURL+"/api/v1/orders", body, data.token)
				r.step = "order"
				return r
			}
		}, nil
	default:
		return nil, fmt.Errorf("unknown scenario %q (use health|products|orders|mixed|realistic)", name)
	}
}

func runLoad(client *http.Client, workers int, duration time.Duration, fn func(*http.Client) result) *stats {
	stop := time.Now().Add(duration)
	var wg sync.WaitGroup
	var mu sync.Mutex
	s := &stats{byStep: make(map[string]*stepStats)}

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(stop) {
				r := fn(client)
				mu.Lock()
				s.record(r)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return s
}

func (s *stats) record(r result) {
	s.total++
	s.latency = append(s.latency, r.elapsed)
	step := r.step
	if step == "" {
		step = "request"
	}
	st, ok := s.byStep[step]
	if !ok {
		st = &stepStats{}
		s.byStep[step] = st
	}
	st.total++
	switch {
	case r.status >= 200 && r.status < 300:
		s.ok++
		st.ok++
	case r.status == http.StatusTooManyRequests:
		s.r429++
	default:
		s.other++
		st.err++
	}
}

func printReport(s *stats, duration time.Duration) {
	fmt.Println("--- HTTP Results ---")
	fmt.Printf("Requests:  %d\n", s.total)
	fmt.Printf("2xx:       %d (%.1f%%)\n", s.ok, pct(s.ok, s.total))
	fmt.Printf("429:       %d (%.1f%%)\n", s.r429, pct(s.r429, s.total))
	fmt.Printf("Errors:    %d (%.1f%%)\n", s.other, pct(s.other, s.total))
	fmt.Printf("Accepted:  %d (2xx + 429)\n", s.accepted())
	if duration > 0 {
		fmt.Printf("Throughput: %.1f req/s\n", float64(s.total)/duration.Seconds())
	}
	if len(s.byStep) > 0 {
		fmt.Println("\n--- By step ---")
		steps := make([]string, 0, len(s.byStep))
		for k := range s.byStep {
			steps = append(steps, k)
		}
		sort.Strings(steps)
		for _, step := range steps {
			st := s.byStep[step]
			fmt.Printf("  %-10s total=%d ok=%d err=%d\n", step+":", st.total, st.ok, st.err)
		}
	}
	if s.r429 > 0 && pct(s.r429, s.total) > 50 {
		fmt.Println("\nNote: high 429 rate — gateway rate limit is active (default 60 req/min per IP).")
		fmt.Println("      Raise RATE_LIMIT_RPM on api-gateway for heavier load tests.")
	}
	if len(s.latency) == 0 {
		return
	}
	sort.Slice(s.latency, func(i, j int) bool { return s.latency[i] < s.latency[j] })
	fmt.Printf("\nLatency p50: %s\n", s.latency[pctIndex(s.latency, 50)])
	fmt.Printf("Latency p95: %s\n", s.latency[pctIndex(s.latency, 95)])
	fmt.Printf("Latency p99: %s\n", s.latency[pctIndex(s.latency, 99)])
	fmt.Printf("Latency max: %s\n", s.latency[len(s.latency)-1])
}

func printJourneyReport(js *journeyStats) {
	fmt.Println("\n--- User journeys ---")
	fmt.Printf("Started:   %d\n", js.started)
	fmt.Printf("Completed: %d (%.1f%%)\n", js.completed, pct(js.completed, js.started))
	fmt.Printf("Failed:    %d (%.1f%%)\n", js.failed, pct(js.failed, js.started))
}

func think(baseMS int) {
	if baseMS <= 0 {
		return
	}
	jitter := rand.Intn(baseMS + 1)
	time.Sleep(time.Duration(baseMS+jitter) * time.Millisecond)
}

func pct(n, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total) * 100
}

func pctIndex(latency []time.Duration, p int) int {
	if len(latency) == 0 {
		return 0
	}
	idx := (len(latency)*p + 99) / 100
	if idx >= len(latency) {
		idx = len(latency) - 1
	}
	return idx
}

func doRequest(client *http.Client, method, url string, body any, token string) result {
	start := time.Now()
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return result{status: 0, elapsed: time.Since(start)}
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := client.Do(req)
	if err != nil {
		return result{status: 0, elapsed: time.Since(start)}
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	return result{status: res.StatusCode, elapsed: time.Since(start)}
}

func postJSON(client *http.Client, url string, body any, token string) (map[string]any, error) {
	b, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("%s status %d: %s", url, res.StatusCode, strings.TrimSpace(string(raw)))
	}
	var env struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	return env.Data, nil
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			return n
		}
	}
	return def
}
