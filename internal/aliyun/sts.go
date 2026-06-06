package aliyun

import (
	"context"

	openapiutil "github.com/alibabacloud-go/darabonba-openapi/v2/utils"
	stsapi "github.com/alibabacloud-go/sts-20150401/v2/client"
)

type officialSTSClient struct {
	client *stsapi.Client
}

func NewSTSClient(options STSOptions) (STSClient, error) {
	if err := validateAccessOptions("sts", options.AccessKeyID, options.AccessKeySecret, options.RegionID); err != nil {
		return nil, err
	}
	cfg := &openapiutil.Config{
		AccessKeyId:     ptr(options.AccessKeyID),
		AccessKeySecret: ptr(options.AccessKeySecret),
		RegionId:        ptr(options.RegionID),
	}
	if options.Endpoint != "" {
		cfg.Endpoint = ptr(options.Endpoint)
	}
	client, err := stsapi.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return &officialSTSClient{client: client}, nil
}

func (c *officialSTSClient) GetCallerIdentity(ctx context.Context) (string, string, error) {
	select {
	case <-ctx.Done():
		return "", "", ctx.Err()
	default:
	}
	resp, err := c.client.GetCallerIdentity()
	if err != nil {
		return "", "", err
	}
	if resp == nil || resp.Body == nil {
		return "", "", ErrNotFound
	}
	return stringValue(resp.Body.AccountId), stringValue(resp.Body.Arn), nil
}
