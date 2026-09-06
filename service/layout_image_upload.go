package service

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
)

var layoutImageFilenameSanitizeRegexp = regexp.MustCompile(`[^A-Za-z0-9._-]`)

func (s *Service) LayoutImageUpload(channelID int64, data []byte, filename, name string, width, height int) (*entity.LayoutImage, error) {
	objectName := layoutImageFilenameSanitizeRegexp.ReplaceAllString(filename, "_")
	if len(objectName) > 80 {
		objectName = objectName[:80]
	}
	if objectName == "" {
		objectName = "image"
	}
	objectName = fmt.Sprintf("%d-%s", time.Now().UnixMilli(), objectName)

	displayName := strings.TrimSpace(name)
	if displayName == "" {
		ext := filepath.Ext(filename)
		displayName = strings.TrimSuffix(filename, ext)
	}
	displayNameRunes := []rune(displayName)
	if len(displayNameRunes) > 200 {
		displayNameRunes = displayNameRunes[:200]
	}
	displayName = string(displayNameRunes)

	url, err := s.DigitalOceaner.SaveLayoutImage(data, channelID, objectName)
	if err != nil {
		return nil, err
	}

	return s.LayoutImageRepositorier.Insert(&entity.LayoutImage{
		ChannelId: channelID,
		Name:      displayName,
		Url:       url,
		Width:     width,
		Height:    height,
		SizeBytes: int64(len(data)),
	})
}
