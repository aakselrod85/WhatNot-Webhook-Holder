package api

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
)

var layoutImageAllowedExtensions = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".webp": true,
}

func (a *API) LayoutImageUpload(w http.ResponseWriter, r *http.Request) (any, error) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		fmt.Println("An error occurred during parsing multipart form of layout image upload: " + err.Error())
		return nil, err
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		fmt.Println("An error occurred during reading file of layout image upload: " + err.Error())
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		fmt.Println("An error occurred during reading file bytes of layout image upload: " + err.Error())
		return nil, err
	}

	if len(data) > 10<<20 {
		err = fmt.Errorf("file is too large: max 10 MB")
		fmt.Println("An error occurred during layout image upload: " + err.Error())
		return nil, err
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !layoutImageAllowedExtensions[ext] {
		err = fmt.Errorf("unsupported file extension: %s", ext)
		fmt.Println("An error occurred during layout image upload: " + err.Error())
		return nil, err
	}

	channelIDStr := r.FormValue("channel_id")
	channelID, err := strconv.ParseInt(channelIDStr, 10, 64)
	if err != nil {
		fmt.Println("An error occurred during parsing channel_id of layout image upload: " + err.Error())
		return nil, err
	}

	return a.Service.LayoutImageUpload(channelID, data, header.Filename)
}
