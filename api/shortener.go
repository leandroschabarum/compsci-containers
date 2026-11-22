package main

import (
	"bytes"
	"context"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
)

// hop guarda a URL e o corpo de cada página do caminho até o destino. Guardar
// o corpo é o que faz a memória crescer rápido no caso "loop", e é de propósito:
// é a falha que a demonstração de OOM provoca.
type hop struct {
	url  string
	body []byte
}

func newFollowClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		// Os redirecionamentos são seguidos à mão em follow, para guardar cada hop.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// follow segue os redirecionamentos de rawURL e devolve a URL final e o número
// de hops. Não há limite de hops: com /shortme?case=loop a lista cresce até o
// kernel matar o processo por falta de memória.
func follow(ctx context.Context, client *http.Client, rawURL string) (string, int, error) {
	var hops []hop
	defer func() { hopsInMemory.Sub(float64(len(hops))) }()

	current := rawURL
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, current, nil)
		if err != nil {
			return "", len(hops), err
		}
		resp, err := client.Do(req)
		if err != nil {
			return "", len(hops), err
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", len(hops), err
		}

		next, err := resp.Location()
		if resp.StatusCode < 300 || resp.StatusCode > 399 || err != nil {
			return current, len(hops), nil
		}
		hops = append(hops, hop{url: current, body: body})
		hopsInMemory.Inc()
		if len(hops)%10 == 0 {
			log.Printf("%d hops em memória seguindo %s", len(hops), rawURL)
		}
		current = next.String()
	}
}

// Cada hop demora hopDelay e ocupa hopSize na memória de quem segue os
// redirecionamentos. Com o lixo que o coletor do Go ainda não liberou, o loop
// cresce uns 5 MB/s e leva a réplica de 96 MB ao OOM em 15 a 20s: dá para ver
// a memória subir no Grafana sem a demonstração ficar parada.
const (
	hopDelay = 90 * time.Millisecond
	hopSize  = 256 << 10
)

// shortme simula um site externo que só redireciona.
//   - case=chain sorteia de 1 a 20 hops e repassa os que faltam em "left".
//   - case=loop redireciona para si mesmo para sempre.
func shortme(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	switch q.Get("case") {
	case "loop":
	case "chain":
		left := rand.IntN(20) + 1
		if v := q.Get("left"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				http.Error(w, "left inválido", http.StatusBadRequest)
				return
			}
			left = n
		}
		if left == 0 {
			w.Write([]byte("destino final\n"))
			return
		}
		q.Set("left", strconv.Itoa(left-1))
	default:
		http.Error(w, "case deve ser chain ou loop", http.StatusBadRequest)
		return
	}

	select {
	case <-time.After(hopDelay):
	case <-r.Context().Done():
		return
	}
	w.Header().Set("Location", "/shortme?"+q.Encode())
	w.WriteHeader(http.StatusFound)
	w.Write(bytes.Repeat([]byte("x"), hopSize))
}
