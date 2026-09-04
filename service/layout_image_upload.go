package service

import (
	"fmt"
	"regexp"
	"time"
)

type LayoutImageUploadResponse struct {
	Url string `json:"url"`
}

var layoutImageFilenameSanitizeRegexp = regexp.MustCompile(`[^A-Za-z0-9._-]`)

func (s *Service) LayoutImageUpload(channelID int64, data []byte, filename string) (*LayoutImageUploadResponse, error) {
	name := layoutImageFilenameSanitizeRegexp.ReplaceAllString(filename, "_")
	if len(name) > 80 {
		name = name[:80]
	}
	if name == "" {
		name = "image"
	}
	name = fmt.Sprintf("%d-%s", time.Now().UnixMilli(), name)

	url, err := s.DigitalOceaner.SaveLayoutImage(data, channelID, name)
	if err != nil {
		return nil, err
	}
	return &LayoutImageUploadResponse{Url: url}, nil
}
