package main

import (
	"log"
	"net/http"
	"time"
)


type Route struct{
	Method string
	Pattern string
	Handler http.HandlerFunc
}

type Router struct{
	routes []Route
}


func (r *Router) Handle(method, pattern string, handler http.HandleFunc){
	r. routes = append(
		r.routes,
		Route{
			Method: method,
			Pattern: pattern,
			Handler: handler
		}
	)
}

func (r *Router) ServerHTTP(res http.ResponseWriter, req *http.Request){
	for _, route := range r.routes{
		if route.Method == req.Method && route.Pattern == req.URL.Path{
			route.Handler(w, req)
		}
	}
	http.NotFound(w, req)
}

func main() {
	log.Println("Creating the splitwise")
	server := &http.Server{
		Addr:           ":8080",
		Handler:        &myHandler{},
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	server.ListenAndServe()
}
