package handlers

import "net/http"


func responseWrapper(StatusCode int, Response []byte, responseWriter http.ResponseWriter){
	responseWriter.WriteHeader(StatusCode)
	responseWriter.Write(Response)
}
