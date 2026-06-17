package aliyun

import (
	"context"
	"encoding/json"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	ecsapi "github.com/alibabacloud-go/ecs-20140526/v4/client"
)

type officialECSClient struct {
	client   *ecsapi.Client
	regionID string
}

func NewECSClient(options ECSOptions) (ECSClient, error) {
	if err := validateAccessOptions("ecs", options.AccessKeyID, options.AccessKeySecret, options.RegionID); err != nil {
		return nil, err
	}
	cfg := &openapi.Config{
		AccessKeyId:     ptr(options.AccessKeyID),
		AccessKeySecret: ptr(options.AccessKeySecret),
		RegionId:        ptr(options.RegionID),
	}
	if options.Endpoint != "" {
		cfg.Endpoint = ptr(options.Endpoint)
	}
	client, err := ecsapi.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return &officialECSClient{client: client, regionID: options.RegionID}, nil
}

func (c *officialECSClient) ListInstances(ctx context.Context) ([]Instance, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	pageNumber := int32(1)
	var instances []Instance
	for {
		req := (&ecsapi.DescribeInstancesRequest{}).
			SetRegionId(c.regionID).
			SetPageNumber(pageNumber).
			SetPageSize(100)
		resp, err := c.client.DescribeInstances(req)
		if err != nil {
			return nil, err
		}
		page := mapInstances(resp)
		instances = append(instances, page...)
		if resp == nil || resp.Body == nil || resp.Body.TotalCount == nil {
			break
		}
		if len(page) == 0 || int32(len(instances)) >= *resp.Body.TotalCount {
			break
		}
		pageNumber++
	}
	return instances, ctx.Err()
}

func (c *officialECSClient) DescribeInstance(ctx context.Context, instanceID string) (Instance, error) {
	if err := ctx.Err(); err != nil {
		return Instance{}, err
	}
	encodedIDs, err := json.Marshal([]string{instanceID})
	if err != nil {
		return Instance{}, err
	}
	req := (&ecsapi.DescribeInstancesRequest{}).
		SetRegionId(c.regionID).
		SetInstanceIds(string(encodedIDs)).
		SetPageSize(1)
	resp, err := c.client.DescribeInstances(req)
	if err != nil {
		return Instance{}, err
	}
	instances := mapInstances(resp)
	if len(instances) == 0 {
		return Instance{}, ErrNotFound
	}
	return instances[0], ctx.Err()
}

func (c *officialECSClient) ListImages(ctx context.Context, ownerAlias string) ([]Image, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	pageNumber := int32(1)
	var images []Image
	for {
		req := describeImagesRequest(c.regionID, ownerAlias, pageNumber)
		resp, err := c.client.DescribeImages(req)
		if err != nil {
			return nil, err
		}
		page := mapImages(resp)
		images = append(images, page...)
		if resp == nil || resp.Body == nil || resp.Body.TotalCount == nil {
			break
		}
		if len(page) == 0 || int32(len(images)) >= *resp.Body.TotalCount {
			break
		}
		pageNumber++
	}
	return images, ctx.Err()
}

func describeImagesRequest(regionID string, ownerAlias string, pageNumber int32) *ecsapi.DescribeImagesRequest {
	req := (&ecsapi.DescribeImagesRequest{}).
		SetRegionId(regionID).
		SetPageNumber(pageNumber).
		SetPageSize(100).
		SetStatus("Creating,Waiting,Available,UnAvailable,CreateFailed,Deprecated")
	if ownerAlias != "" && ownerAlias != "all" {
		req.SetImageOwnerAlias(ownerAlias)
	}
	return req
}

func (c *officialECSClient) ImportImage(ctx context.Context, input ImportImageInput) (string, string, string, error) {
	if err := ctx.Err(); err != nil {
		return "", "", "", err
	}
	regionID := firstString(input.RegionID, c.regionID)
	req := (&ecsapi.ImportImageRequest{}).
		SetRegionId(regionID).
		SetArchitecture(firstString(input.Architecture, "x86_64")).
		SetOSType(firstString(input.OSType, "linux")).
		SetPlatform(firstString(input.Platform, "Others Linux")).
		SetDiskDeviceMapping([]*ecsapi.ImportImageRequestDiskDeviceMapping{
			importDiskMapping(input),
		})
	if input.ImageName != "" {
		req.SetImageName(input.ImageName)
	}
	if input.Description != "" {
		req.SetDescription(input.Description)
	}
	if input.BootMode != "" {
		req.SetBootMode(input.BootMode)
	}
	if input.RoleName != "" {
		req.SetRoleName(input.RoleName)
	}
	if input.ClientToken != "" {
		req.SetClientToken(input.ClientToken)
	}
	resp, err := c.client.ImportImage(req)
	if err != nil {
		return "", "", "", err
	}
	if resp == nil || resp.Body == nil {
		return "", "", "", ErrNotFound
	}
	return stringValue(resp.Body.ImageId), stringValue(resp.Body.TaskId), stringValue(resp.Body.RequestId), ctx.Err()
}

func (c *officialECSClient) DescribeTask(ctx context.Context, taskID string) (TaskStatus, error) {
	if err := ctx.Err(); err != nil {
		return TaskStatus{}, err
	}
	req := (&ecsapi.DescribeTasksRequest{}).
		SetRegionId(c.regionID).
		SetTaskIds(taskID).
		SetPageSize(100)
	resp, err := c.client.DescribeTasks(req)
	if err != nil {
		return TaskStatus{}, err
	}
	if resp == nil || resp.Body == nil || resp.Body.TaskSet == nil {
		return TaskStatus{}, ErrNotFound
	}
	for _, task := range resp.Body.TaskSet.Task {
		if task == nil {
			continue
		}
		if stringValue(task.TaskId) == taskID {
			return TaskStatus{
				ID:           stringValue(task.TaskId),
				ResourceID:   stringValue(task.ResourceId),
				Action:       stringValue(task.TaskAction),
				Status:       stringValue(task.TaskStatus),
				CreationTime: stringValue(task.CreationTime),
				FinishedTime: stringValue(task.FinishedTime),
			}, ctx.Err()
		}
	}
	return TaskStatus{}, ErrNotFound
}

func (c *officialECSClient) StopInstance(ctx context.Context, instanceID string, force bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := c.client.StopInstance((&ecsapi.StopInstanceRequest{}).
		SetInstanceId(instanceID).
		SetForceStop(force))
	return err
}

func (c *officialECSClient) StartInstance(ctx context.Context, instanceID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := c.client.StartInstance((&ecsapi.StartInstanceRequest{}).SetInstanceId(instanceID))
	return err
}

func (c *officialECSClient) ReplaceSystemDisk(ctx context.Context, instanceID string, imageID string) (string, string, error) {
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	resp, err := c.client.ReplaceSystemDisk((&ecsapi.ReplaceSystemDiskRequest{}).
		SetInstanceId(instanceID).
		SetImageId(imageID))
	if err != nil {
		return "", "", err
	}
	if resp == nil || resp.Body == nil {
		return "", "", ErrNotFound
	}
	return stringValue(resp.Body.DiskId), stringValue(resp.Body.RequestId), ctx.Err()
}

func importDiskMapping(input ImportImageInput) *ecsapi.ImportImageRequestDiskDeviceMapping {
	mapping := (&ecsapi.ImportImageRequestDiskDeviceMapping{}).
		SetFormat("QCOW2").
		SetOSSBucket(input.OSSBucket).
		SetOSSObject(input.OSSObject)
	if input.DiskImageSizeGiB > 0 {
		mapping.SetDiskImageSize(input.DiskImageSizeGiB)
	}
	return mapping
}

func mapInstances(resp *ecsapi.DescribeInstancesResponse) []Instance {
	if resp == nil || resp.Body == nil || resp.Body.Instances == nil {
		return nil
	}
	instances := make([]Instance, 0, len(resp.Body.Instances.Instance))
	for _, item := range resp.Body.Instances.Instance {
		if item == nil {
			continue
		}
		instances = append(instances, Instance{
			ID:         stringValue(item.InstanceId),
			Name:       stringValue(item.InstanceName),
			Status:     stringValue(item.Status),
			ImageID:    stringValue(item.ImageId),
			Type:       stringValue(item.InstanceType),
			ZoneID:     stringValue(item.ZoneId),
			RegionID:   stringValue(item.RegionId),
			PublicIP:   firstIP(item.PublicIpAddress),
			PrivateIP:  firstPrivateIP(item.VpcAttributes),
			CreateTime: stringValue(item.CreationTime),
		})
	}
	return instances
}

func mapImages(resp *ecsapi.DescribeImagesResponse) []Image {
	if resp == nil || resp.Body == nil || resp.Body.Images == nil {
		return nil
	}
	images := make([]Image, 0, len(resp.Body.Images.Image))
	for _, item := range resp.Body.Images.Image {
		if item == nil {
			continue
		}
		sourceBucket, sourceObject, sourceFormat := firstImageImportSource(item.DiskDeviceMappings)
		images = append(images, Image{
			ID:              stringValue(item.ImageId),
			Name:            stringValue(item.ImageName),
			Status:          stringValue(item.Status),
			Progress:        stringValue(item.Progress),
			CreationTime:    stringValue(item.CreationTime),
			OwnerAlias:      stringValue(item.ImageOwnerAlias),
			Platform:        stringValue(item.Platform),
			Architecture:    stringValue(item.Architecture),
			OSType:          stringValue(item.OSType),
			SizeGiB:         int32Value(item.Size),
			Usage:           stringValue(item.Usage),
			SourceOSSBucket: sourceBucket,
			SourceOSSObject: sourceObject,
			SourceFormat:    sourceFormat,
		})
	}
	return images
}

func firstImageImportSource(mappings *ecsapi.DescribeImagesResponseBodyImagesImageDiskDeviceMappings) (string, string, string) {
	if mappings == nil {
		return "", "", ""
	}
	for _, mapping := range mappings.DiskDeviceMapping {
		if mapping == nil {
			continue
		}
		bucket := stringValue(mapping.ImportOSSBucket)
		object := stringValue(mapping.ImportOSSObject)
		if bucket != "" || object != "" {
			return bucket, object, stringValue(mapping.Format)
		}
	}
	return "", "", ""
}

func firstIP(value *ecsapi.DescribeInstancesResponseBodyInstancesInstancePublicIpAddress) string {
	if value == nil {
		return ""
	}
	return firstPtrString(value.IpAddress)
}

func firstPrivateIP(value *ecsapi.DescribeInstancesResponseBodyInstancesInstanceVpcAttributes) string {
	if value == nil || value.PrivateIpAddress == nil {
		return ""
	}
	return firstPtrString(value.PrivateIpAddress.IpAddress)
}
