package service

import "bytes"

type DigitalOceaner interface {
	SaveLabel(bytes.Buffer, string) (string, error)
	SaveCardPhoto(data []byte, seriesID int64, filename string) (string, error)
	SaveCardThumbnail(data []byte, seriesID int64, filename string) (string, error)
}
