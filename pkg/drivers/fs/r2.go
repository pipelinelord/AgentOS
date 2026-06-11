package fs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type R2Driver struct {
	client *s3.Client
	bucket string
}

func NewR2Driver(ctx context.Context) (*R2Driver, error) {
	accountId := os.Getenv("R2_ACCOUNT_ID")
	accessKey := os.Getenv("R2_ACCESS_KEY_ID")
	secretKey := os.Getenv("R2_SECRET_ACCESS_KEY")
	bucket := os.Getenv("R2_BUCKET")

	if accountId == "" || accessKey == "" || secretKey == "" || bucket == "" {
		return nil, fmt.Errorf("R2 environment variables not fully set")
	}

	r2Resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL: fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountId),
		}, nil
	})

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithEndpointResolverWithOptions(r2Resolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		config.WithRegion("auto"),
	)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(cfg)

	return &R2Driver{
		client: client,
		bucket: bucket,
	}, nil
}

func (d *R2Driver) Write(ctx context.Context, path string, data string) (int, error) {
	input := &s3.PutObjectInput{
		Bucket: aws.String(d.bucket),
		Key:    aws.String(path),
		Body:   bytes.NewReader([]byte(data)),
	}

	_, err := d.client.PutObject(ctx, input)
	if err != nil {
		return 0, err
	}
	return len(data), nil
}

func (d *R2Driver) Read(ctx context.Context, path string) (string, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(d.bucket),
		Key:    aws.String(path),
	}

	out, err := d.client.GetObject(ctx, input)
	if err != nil {
		return "", err
	}
	defer out.Body.Close()

	body, err := io.ReadAll(out.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
