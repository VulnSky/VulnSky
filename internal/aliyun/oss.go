package aliyun

import (
	"context"
	"errors"
	"net/http"
	"time"

	ossapi "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
)

type officialOSSClient struct {
	client *ossapi.Client
}

func NewOSSClient(options OSSOptions) (OSSClient, error) {
	if err := validateAccessOptions("oss", options.AccessKeyID, options.AccessKeySecret, options.RegionID); err != nil {
		return nil, err
	}
	if options.Endpoint == "" {
		return nil, errors.New("missing endpoint option for oss")
	}
	cfg := ossapi.LoadDefaultConfig().
		WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			options.AccessKeyID,
			options.AccessKeySecret,
			options.SecurityToken,
		)).
		WithRegion(options.RegionID).
		WithEndpoint(options.Endpoint)

	return &officialOSSClient{client: ossapi.NewClient(cfg)}, nil
}

func (c *officialOSSClient) ListObjects(ctx context.Context, bucket string, prefix string) ([]string, error) {
	var objects []string
	var token *string
	for {
		req := &ossapi.ListObjectsV2Request{
			Bucket: ossapi.Ptr(bucket),
			Prefix: ossapi.Ptr(prefix),
		}
		if token != nil {
			req.ContinuationToken = token
		}
		result, err := c.client.ListObjectsV2(ctx, req)
		if err != nil {
			return nil, err
		}
		for _, object := range result.Contents {
			key := stringValue(object.Key)
			if key != "" {
				objects = append(objects, key)
			}
		}
		if !result.IsTruncated || result.NextContinuationToken == nil {
			break
		}
		token = result.NextContinuationToken
	}
	return objects, nil
}

func (c *officialOSSClient) ObjectExists(ctx context.Context, bucket string, key string) (bool, error) {
	_, err := c.client.HeadObject(ctx, &ossapi.HeadObjectRequest{
		Bucket: ossapi.Ptr(bucket),
		Key:    ossapi.Ptr(key),
	})
	if err == nil {
		return true, nil
	}
	var serviceErr *ossapi.ServiceError
	if errors.As(err, &serviceErr) && serviceErr.HttpStatusCode() == http.StatusNotFound {
		return false, nil
	}
	return false, err
}

func (c *officialOSSClient) UploadFile(ctx context.Context, bucket string, key string, path string, onProgress func(done, total int64)) (string, error) {
	result, err := c.client.PutObjectFromFile(ctx, &ossapi.PutObjectRequest{
		Bucket: ossapi.Ptr(bucket),
		Key:    ossapi.Ptr(key),
		ProgressFn: func(_ int64, transferred int64, total int64) {
			if onProgress != nil {
				onProgress(transferred, total)
			}
		},
	}, path)
	if err != nil {
		return "", err
	}
	if result == nil || result.Headers == nil {
		return "", nil
	}
	return result.Headers.Get("x-oss-request-id"), nil
}

func (c *officialOSSClient) PresignGet(ctx context.Context, bucket string, key string, expires time.Duration) (string, error) {
	result, err := c.client.Presign(ctx, &ossapi.GetObjectRequest{
		Bucket: ossapi.Ptr(bucket),
		Key:    ossapi.Ptr(key),
	}, ossapi.PresignExpires(expires))
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", nil
	}
	return result.URL, nil
}

func (c *officialOSSClient) GetBucketLocation(ctx context.Context, bucket string) (string, error) {
	result, err := c.client.GetBucketLocation(ctx, &ossapi.GetBucketLocationRequest{
		Bucket: ossapi.Ptr(bucket),
	})
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", ErrNotFound
	}
	return NormalizeOSSLocation(stringValue(result.LocationConstraint)), nil
}
