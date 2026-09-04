package digital_ocean

import (
	"bytes"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type DigitalOcean struct {
	s3Client  *s3.S3
	spacesURL string
}

func InitDigitalOcean(key, secret, endpoint, region, spacesURL string) *DigitalOcean {
	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(key, secret, ""),
		Endpoint:         aws.String(endpoint),
		Region:           aws.String(region),
		S3ForcePathStyle: aws.Bool(false),
	}

	newSession, err := session.NewSession(s3Config)
	if err != nil {
		panic(err)
	}
	s3Client := s3.New(newSession)
	return &DigitalOcean{s3Client, spacesURL}
}

func (d *DigitalOcean) saveObject(data []byte, filePath string) (string, error) {
	object := s3.PutObjectInput{
		Bucket: aws.String("mount-olympus-storage"),
		Key:    aws.String(filePath),
		Body:   bytes.NewReader(data),
		ACL:    aws.String("public-read"),
		Metadata: map[string]*string{
			"x-amz-meta-success": aws.String("ok"),
		},
	}
	_, err := d.s3Client.PutObject(&object)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s", d.spacesURL, filePath), nil
}

func (d *DigitalOcean) SaveCardPhoto(data []byte, seriesID int64, filename string) (string, error) {
	return d.saveObject(data, fmt.Sprintf("cards/%d/%s", seriesID, filename))
}

func (d *DigitalOcean) SaveCardThumbnail(data []byte, seriesID int64, filename string) (string, error) {
	return d.saveObject(data, fmt.Sprintf("thumbnail/%d/%s", seriesID, filename))
}

func (d *DigitalOcean) SaveLayoutImage(data []byte, channelID int64, filename string) (string, error) {
	return d.saveObject(data, fmt.Sprintf("layout/%d/%s", channelID, filename))
}

func (d *DigitalOcean) SaveLabel(buffer bytes.Buffer, name string) (string, error) {
	reader := bytes.NewReader(buffer.Bytes())
	filePath := fmt.Sprintf("labels/%s", name)
	object := s3.PutObjectInput{
		Bucket: aws.String("mount-olympus-storage"), // The path to the directory you want to upload the object to, starting with your Space name.
		Key:    aws.String(filePath),                // Object key, referenced whenever you want to access this file later.
		Body:   reader,                              // The object's contents.
		ACL:    aws.String("public-read"),           // Defines Access-control List (ACL) permissions, such as private or public.
		Metadata: map[string]*string{ // Required. Defines metadata tags.
			"x-amz-meta-success": aws.String("ok"),
		},
	}

	_, err := d.s3Client.PutObject(&object)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s", d.spacesURL, filePath), nil
}
