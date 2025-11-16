package s3api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/TecharoHQ/anubis/lib/store"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	ErrNoRegion          = errors.New("s3api.Config: no region env var name defined")
	ErrNoAccessKeyID     = errors.New("s3api.Config: no access key id env var name defined")
	ErrNoSecretAccessKey = errors.New("s3api.Config: no secret access key env var name defined")
	ErrNoBucketName      = errors.New("s3api.Config: no bucket name env var name defined")
)

func init() {
	store.Register("s3api", Factory{})
}

// S3API is the subset of the AWS S3 client used by this store. It enables mocking in tests.
type S3API interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
}

// Factory builds an S3-backed store. Tests can inject a Mock via Client.
// Factory can optionally carry a preconstructed S3 client (e.g., a mock in tests).
type Factory struct {
	Client S3API
}

func (f Factory) Build(ctx context.Context, data json.RawMessage) (store.Interface, error) {
	var config Config

	if err := json.Unmarshal([]byte(data), &config); err != nil {
		return nil, fmt.Errorf("%w: %w", store.ErrBadConfig, err)
	}

	if err := config.Valid(); err != nil {
		return nil, fmt.Errorf("%w: %w", store.ErrBadConfig, err)
	}

	if config.BucketName == "" {
		return nil, fmt.Errorf("%w: %s", store.ErrBadConfig, ErrNoBucketName)
	}

	// If a client was injected (e.g., tests), use it directly.
	if f.Client != nil {
		return &Store{
			s3:     f.Client,
			bucket: config.BucketName,
		}, nil
	}

	cfg, err := awsConfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("can't load AWS config from environment: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = config.PathStyle
	})

	return &Store{
		s3:     client,
		bucket: config.BucketName,
	}, nil
}

func (Factory) Valid(data json.RawMessage) error {
	var config Config
	if err := json.Unmarshal([]byte(data), &config); err != nil {
		return fmt.Errorf("%w: %w", store.ErrBadConfig, err)
	}

	if err := config.Valid(); err != nil {
		return fmt.Errorf("%w: %w", store.ErrBadConfig, err)
	}

	return nil
}

type Config struct {
	BucketName string `json:"bucketName"`
	PathStyle  bool   `json:"pathStyle"`
}

func (c Config) Valid() error {
	var errs []error

	if c.BucketName == "" {
		errs = append(errs, ErrNoBucketName)
	}

	if len(errs) != 0 {
		return fmt.Errorf("s3api.Config: invalid config: %w", errors.Join(errs...))
	}

	return nil
}
