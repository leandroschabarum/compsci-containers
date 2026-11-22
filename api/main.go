package main

import (
	"cmp"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	requests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "shortener_http_requests_total",
		Help: "Requisições HTTP atendidas, por rota e status.",
	}, []string{"route", "code"})
	latency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "shortener_http_request_duration_seconds",
		Help: "Tempo de resposta das requisições HTTP, por rota.",
	}, []string{"route"})
	hopsInMemory = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "shortener_hops_in_memory",
		Help: "Hops guardados em memória por encurtamentos em andamento.",
	})
)

type api struct {
	store    *store
	client   *http.Client
	instance string
	chaos    bool
}

func main() {
	hostname, _ := os.Hostname()
	instance := cmp.Or(os.Getenv("INSTANCE"), hostname)
	chaos := os.Getenv("CHAOS_ENABLED") == "true"

	st, err := openStore(cmp.Or(os.Getenv("DB_PATH"), "/data/links.db"))
	if err != nil {
		log.Fatalf("abrindo o banco: %v", err)
	}

	a := &api{store: st, client: newFollowClient(), instance: instance, chaos: chaos}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /links", a.createLink)
	mux.HandleFunc("GET /links", a.listLinks)
	mux.HandleFunc("GET /{code}", a.redirect)
	mux.HandleFunc("GET /health", a.health)
	mux.Handle("GET /metrics", promhttp.Handler())
	if chaos {
		mux.HandleFunc("GET /shortme", shortme)
	}

	addr := cmp.Or(os.Getenv("ADDR"), ":8080")
	log.Printf("%s ouvindo em %s (chaos=%v)", instance, addr, chaos)
	log.Fatal(http.ListenAndServe(addr, a.instrument(mux)))
}

func (a *api) createLink(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !isHTTPURL(req.URL) {
		http.Error(w, "informe uma url http ou https válida", http.StatusBadRequest)
		return
	}

	target, hops := req.URL, 0
	// Seguir URLs enviadas pelo usuário é SSRF num serviço real, por isso só
	// acontece no modo de demonstração.
	if a.chaos {
		final, n, err := follow(r.Context(), a.client, req.URL)
		if err != nil {
			log.Printf("seguindo %s: %v", req.URL, err)
			http.Error(w, "falha ao seguir os redirecionamentos", http.StatusBadGateway)
			return
		}
		log.Printf("%s resolvido em %d hops: %s", req.URL, n, final)
		target, hops = final, n
	}

	l, err := a.store.create(r.Context(), target, hops)
	if err != nil {
		log.Printf("salvando link: %v", err)
		http.Error(w, "falha ao salvar o link", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, l)
}

func (a *api) listLinks(w http.ResponseWriter, r *http.Request) {
	links, err := a.store.list(r.Context(), 20)
	if err != nil {
		log.Printf("listando links: %v", err)
		http.Error(w, "falha ao listar os links", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, links)
}

func (a *api) redirect(w http.ResponseWriter, r *http.Request) {
	l, err := a.store.get(r.Context(), r.PathValue("code"))
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("buscando link: %v", err)
		http.Error(w, "falha ao buscar o link", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, l.URL, http.StatusFound)
}

func (a *api) health(w http.ResponseWriter, r *http.Request) {
	if err := a.store.db.PingContext(r.Context()); err != nil {
		log.Printf("healthcheck: %v", err)
		http.Error(w, "banco indisponível", http.StatusServiceUnavailable)
		return
	}
	w.Write([]byte("ok\n"))
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// instrument marca cada resposta com o nome da réplica e registra as métricas.
// A rota vem de r.Pattern, que o ServeMux preenche, para que /{code} não crie
// uma série nova no Prometheus para cada link.
func (a *api) instrument(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		w.Header().Set("X-Instance", a.instance)
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		route := cmp.Or(r.Pattern, "none")
		requests.WithLabelValues(route, strconv.Itoa(rec.status)).Inc()
		latency.WithLabelValues(route).Observe(time.Since(start).Seconds())
	})
}

func isHTTPURL(raw string) bool {
	u, err := url.ParseRequestURI(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("escrevendo resposta: %v", err)
	}
}
