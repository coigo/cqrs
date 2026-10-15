package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"
)

const url = "http://localhost:8080/payment"

type Pagamento struct {
	TipoEvento  string  `json:"tipo_evento"`
	UsuarioNome string  `json:"usuario_nome"`
	Valor       int `json:"valor"`
}

var usuarios = []string{"Fernando", "Luciado", "Ana", "Carlos"}

func worker(ctx context.Context, wg *sync.WaitGroup, client *http.Client, nome string) {
	defer wg.Done()

	// gerador próprio por goroutine (o rand global tem lock, e assim evita contenção)
	rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(len(nome))))

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// valor aleatório entre -500 e 1000, com 2 casas decimais
		valor := rng.Intn(1501) - 500 

		body, err := json.Marshal(Pagamento{
			TipoEvento:  "PAGAMENTO",
			UsuarioNome: nome,
			Valor:       valor,
		})
		if err != nil {
			log.Printf("[%s] erro no json: %v", nome, err)
			continue
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			log.Printf("[%s] erro criando request: %v", nome, err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return // Ctrl+C
			}
			log.Printf("[%s] erro na chamada: %v", nome, err)
		} else {
			io.Copy(io.Discard, resp.Body) // drena o body para reaproveitar a conexão
			resp.Body.Close()
			fmt.Printf("[%s] valor=%8.2f -> %s\n", nome, valor, resp.Status)
		}

		// pausa aleatória entre 100ms e 500ms
		time.Sleep(time.Duration(100+rng.Intn(400)) * time.Millisecond)
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	client := &http.Client{Timeout: 5 * time.Second}

	var wg sync.WaitGroup
	for _, nome := range usuarios {
		wg.Add(1)
		go worker(ctx, &wg, client, nome)
	}

	wg.Wait()
	fmt.Println("encerrado")
}