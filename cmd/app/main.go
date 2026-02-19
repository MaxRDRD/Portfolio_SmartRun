package app

import (
	db "SmartRun/internal/DB"
	"SmartRun/internal/server"
	"SmartRun/internal/user"
	"context"
	"log"
	"net/http"
	"os"
)

func main() {
	ctx := context.Background()
	conn_string := os.Getenv("CONN_STRING")
	pool, err := db.NewPool(ctx, conn_string)
	if err != nil {
		panic(err)
	}

	userRepo := user.NewRepository(pool)
	userService := user.NewService(userRepo)
	userHandler := user.NewHandler(userService)

	server := server.NewServer(userHandler)

	log.Fatal(http.ListenAndServe(":8080", server))
}
