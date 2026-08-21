package api

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
)

func (a *API) PhotoUpdateImage(w http.ResponseWriter, r *http.Request) (any, error) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		fmt.Println("An error occurred during parsing multipart form of photo update-image: " + err.Error())
		return nil, err
	}

	id, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err != nil {
		fmt.Println("An error occurred during parsing id of photo update-image: " + err.Error())
		return nil, err
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		fmt.Println("An error occurred during reading file of photo update-image: " + err.Error())
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		fmt.Println("An error occurred during reading file bytes of photo update-image: " + err.Error())
		return nil, err
	}

	return a.Service.PhotoUpdateImage(id, data, header.Filename)
}
