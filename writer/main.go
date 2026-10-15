package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)
type App struct {
	db *pgxpool.Pool
}

type WritePayload struct {
	TipoEvento 		string `json:"tipo_evento"`
	UsuarioNome 	string `json:"usuario_nome"`
	Valor				int64 `json:"valor"`
}

func (a *App) Write (ctx *gin.Context) {
	var payload WritePayload
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "wrong format paylaod"})
		return
	}

	fmt.Println(payload)
	
	_, err := a.db.Exec(ctx, "INSERT INTO events (tipo_evento, usuario_nome,valor ) VALUES ($1,$2,$3)", payload.TipoEvento, payload.UsuarioNome, payload.Valor)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "debt not paid :("})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"message": "paid"})
	
}

func main() {
	ctx := context.Background()

	db, err := pgxpool.New(ctx, "postgres://postgres:postgres@localhost:5432/cqrs_writer")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	db.Exec(ctx, `CREATE TABLE IF NOT EXISTS events (
	    id             BIGSERIAL PRIMARY KEY,
	    tipo_evento    TEXT NOT NULL,
	    usuario_nome   TEXT NOT NULL,
	    valor          INT NOT NULL DEFAULT 0,
	    criado_em      TIMESTAMPTZ NOT NULL DEFAULT now()
   );`)

	app := &App{db}

	r := gin.Default()
	r.POST("/payment", app.Write)

	r.Run(":8080")
	
}
