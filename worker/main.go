package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type App struct {
	cache *redis.Client
}

type Event struct {
	ID          int64     `json:"id"`
	TipoEvento  string    `json:"tipo_evento"`
	UsuarioNome string    `json:"usuario_nome"`
	Valor       int64     `json:"valor"`
	CriadoEm    time.Time `json:"criado_em"`
}

type Change struct {
	Before *Event `json:"before"`
	After  *Event `json:"after"`
	Op     string `json:"op"` // c=insert, u=update, d=delete, r=snapshot
}

type ProcessRecord struct {
	UsuarioNome  string    `json:"usuario_nome"`
	Saldo        int64     `json:"saldo"`
	UltimoEvento int64     `json:"ultimo_evento"`
	AtualizadoEm time.Time `json:"atualizado_em"`
}

func (a *App) Process(ctx *gin.Context) {
	var payload Change
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "erro processando"})
		return
	}

	if payload.Op != "c" && payload.Op != "r" {
		ctx.Status(http.StatusOK)
		return
	}

	after := payload.After
	chave := "usuario:" + after.UsuarioNome
	reqCtx := ctx.Request.Context()

	var record ProcessRecord
	raw, err := a.cache.Get(reqCtx, chave).Result()
	switch {
	case err == redis.Nil:
		fmt.Println("salvando novo")
		record = ProcessRecord{
			UsuarioNome:  after.UsuarioNome,
			Saldo:        after.Valor,
			UltimoEvento: after.ID,
			AtualizadoEm: after.CriadoEm,
		}

	case err != nil:
		fmt.Println("process:", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": "erro lendo cache"})
		return

	default:
		if err := json.Unmarshal([]byte(raw), &record); err != nil {
			fmt.Println("process:", err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"message": "erro processando json"})
			return
		}
		if after.ID <= record.UltimoEvento { // já processado
			ctx.Status(http.StatusOK)
			return
		}
		fmt.Println("atualizando")
		record.Saldo += after.Valor
		record.UltimoEvento = after.ID
		record.AtualizadoEm = after.CriadoEm
	}

	write, err := json.Marshal(record)
	if err != nil {
		fmt.Println("process:", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": "erro processando json"})
		return
	}

	if err := a.cache.Set(reqCtx, chave, write, 0).Err(); err != nil {
		fmt.Println("process:", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": "erro atualizando"})
		return
	}

	log.Printf("novo evento: %s de %s, R$ %.2f", after.TipoEvento, after.UsuarioNome, after.Valor)
	ctx.Status(http.StatusOK)
}

func (a *App) Read (ctx *gin.Context) {
	reqCtx := ctx.Request.Context()

	var chaves []string
	iter := a.cache.Scan(reqCtx, 0, "usuario:*", 100).Iterator()
	for iter.Next(reqCtx) {
		chaves = append(chaves, iter.Val())
	}
	if err := iter.Err(); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"erro": err.Error()})
		return
	}
	if len(chaves) == 0 {
		ctx.JSON(http.StatusOK, []ProcessRecord{})
		return
	}
	
	valores, err := a.cache.MGet(reqCtx, chaves...).Result()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"erro": err.Error()})
		return
	}
	
	registros := make([]ProcessRecord, 0, len(valores))
	for i, v := range valores {
		raw, ok := v.(string)
		if !ok { // nil: a chave sumiu entre o Scan e o MGet
			continue
		}
	
		var r ProcessRecord
		if err := json.Unmarshal([]byte(raw), &r); err != nil {
			log.Printf("json inválido em %s: %v", chaves[i], err)
			continue
		}
		registros = append(registros, r)
	}
	ctx.JSON(http.StatusOK, registros)
	
}

func main() {

	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer rdb.Close()

	a := App{cache: rdb}

	r := gin.Default()
	r.POST("/process", a.Process)
	r.GET("/read", a.Read)
	r.Run(":8081")
}
