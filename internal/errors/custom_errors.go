package customerrors

import (
	"fmt"
	"net/http"
)

type CustomError struct {
	Message string
	Status  http.ConnState
}

func (e *CustomError) Error() string {
	return e.Message
}

func (e *CustomError) ReportError(w http.ResponseWriter) {
	w.WriteHeader(int(e.Status))
	status, err := w.Write([]byte(e.Message))
	if err != nil {
		fmt.Printf("Problem with writing to responser status is %d: %s\n", status, err.Error())
	}
}
