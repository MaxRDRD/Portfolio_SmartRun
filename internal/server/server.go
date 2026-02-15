package server

import (
	"errors"
	"net/http"
)

func StartHTTPServer() error {

	/*
		http.HandleFunc("/user", UserProfile)
		http.HandleFunc("/user/auth", UserAuth)
		http.HandleFunc("/user/authn ", UserAutentification)
		http.HandleFunc("/ ", MainPage)
		http.HandleFunc("/workout ", WorkoutPage)
	*/

	err := http.ListenAndServe(":5050", nil)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}
