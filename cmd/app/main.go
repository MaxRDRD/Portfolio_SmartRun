package app

import (
	"SmartRun/internal/server"
	"SmartRun/internal/user"
	"log"
	"net/http"
)

func main() {

	db := db.NewPool()
	userRepo := user.NewRepository(db)
	userService := user.NewService(userRepo)
	userHandler := user.NewHandler(userService)

	server := server.NewServer(userHandler)

	log.Fatal(http.ListenAndServe(":8080", server))
}
