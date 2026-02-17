package controller

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	r := chi.NewRouter()

	// Стандартные middleware (важно соблюдать порядок!)
	r.Use(middleware.RequestID) // Добавляет RequestID в каждый запрос
	r.Use(middleware.RealIP)    // Определяет реальный IP через заголовки
	r.Use(middleware.Logger)    // Логирует запросы
	r.Use(middleware.Recoverer) // Ловит паники

	// Таймаут на выполнение запроса
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/me", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		w.Write([]byte("User ID: " + id))
	})
	http.ListenAndServe(":8080", r)
}
