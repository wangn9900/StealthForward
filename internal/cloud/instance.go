package cloud

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/wangn9900/StealthForward/internal/database"
	"github.com/wangn9900/StealthForward/internal/models"
)

type CreateInstanceRequest struct {
	Region       string `json:"region"`
	InstanceType string `json:"instance_type"` // e.g. t3.micro
	ImageID      string `json:"image_id"`      // Optional: Specific AMI ID
	RootPassword string `json:"root_password"`
}

type CreateInstanceResponse struct {
	InstanceID string `json:"instance_id"`
	PublicIP   string `json:"public_ip"`
}

// ProvisionInstance 创建新实例 (Debian 12 + Root Login)
func ProvisionInstance(ctx context.Context, req CreateInstanceRequest) (*CreateInstanceResponse, error) {
	// 1. Load Config
	cfg, err := loadAWSConfig(ctx, req.Region)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	// 2. Determine AMI
	amiID := req.ImageID
	if amiID == "" {
		// Fallback to auto-find Debian 12 if not specified
		amiID, err = findDebianAMI(ctx, client)
		if err != nil {
			return nil, fmt.Errorf("failed to find default AMI: %v", err)
		}
		log.Printf("[Cloud] Auto-selected AMI: %s", amiID)
	} else {
		log.Printf("[Cloud] Using specified AMI: %s", amiID)
	}

	// 3. Prepare Key Pair
	// We create a unique key per region per project to avoid conflicts, or just one global key.
	// User requested to SAVE the key locally as a fallback.
	keyName := fmt.Sprintf("StealthKey_%s", req.Region)
	keyOut, err := client.CreateKeyPair(ctx, &ec2.CreateKeyPairInput{KeyName: aws.String(keyName)})
	if err != nil {
		// If key already exists, we assume we have it locally or user maintains it.
		// If it's a conflict, just log and use existing.
		if !strings.Contains(err.Error(), "InvalidKeyPair.Duplicate") {
			log.Printf("[Cloud] KeyPair create error (might exist): %v", err)
		} else {
			log.Printf("[Cloud] Using existing key pair: %s", keyName)
		}
	} else if keyOut.KeyMaterial != nil {
		// Save the new key material locally
		keyPath := fmt.Sprintf("store/keys/%s.pem", keyName)
		if saveErr := os.WriteFile(keyPath, []byte(*keyOut.KeyMaterial), 0600); saveErr != nil {
			log.Printf("[Cloud] Failed to save key material to %s: %v", keyPath, saveErr)
		} else {
			log.Printf("[Cloud] Key saved to: %s", keyPath)
		}
	}

	// 4. Prepare Security Group
	sgID, err := ensureSecurityGroup(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to setup security group: %v", err)
	}

	// 5. Generate UserData
	userData := generateUserData(req.RootPassword)
	encodedUserData := base64.StdEncoding.EncodeToString([]byte(userData))

	// 6. Run Instances
	runOut, err := client.RunInstances(ctx, &ec2.RunInstancesInput{
		ImageId:          aws.String(amiID),
		InstanceType:     types.InstanceType(req.InstanceType),
		KeyName:          aws.String(keyName),
		MinCount:         aws.Int32(1),
		MaxCount:         aws.Int32(1),
		SecurityGroupIds: []string{sgID},
		UserData:         aws.String(encodedUserData),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeInstance,
				Tags: []types.Tag{
					{Key: aws.String("Name"), Value: aws.String("Stealth-Node-" + req.Region)},
					{Key: aws.String("CreatedBy"), Value: aws.String("StealthController")},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("RunInstances failed: %v", err)
	}

	instanceID := *runOut.Instances[0].InstanceId
	log.Printf("[Cloud] Instance launched: %s, waiting for IP...", instanceID)

	// 7. Wait for Running & IP
	waiter := ec2.NewInstanceRunningWaiter(client)
	err = waiter.Wait(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}, 2*time.Minute) // Wait up to 2 minutes
	if err != nil {
		return nil, fmt.Errorf("wait for instance running failed: %v", err)
	}

	// Get Public IP
	desc, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return nil, fmt.Errorf("describe instance failed: %v", err)
	}

	publicIP := ""
	if len(desc.Reservations) > 0 && len(desc.Reservations[0].Instances) > 0 {
		inst := desc.Reservations[0].Instances[0]
		if inst.PublicIpAddress != nil {
			publicIP = *inst.PublicIpAddress
		}
	}

	return &CreateInstanceResponse{
		InstanceID: instanceID,
		PublicIP:   publicIP,
	}, nil
}

// TerminateInstance 销毁实例
func TerminateInstance(ctx context.Context, region, instanceID string) error {
	cfg, err := loadAWSConfig(ctx, region)
	if err != nil {
		return err
	}
	client := ec2.NewFromConfig(cfg)

	_, err = client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: []string{instanceID},
	})
	return err
}

// Helper: Load AWS Config
func loadAWSConfig(ctx context.Context, region string) (aws.Config, error) {
	// DB -> Env
	var settings []models.SystemSetting
	configMap := make(map[string]string)
	if err := database.DB.Find(&settings).Error; err == nil {
		for _, s := range settings {
			configMap[s.Key] = s.Value
		}
	}

	ak := configMap[models.ConfigKeyAwsAccessKeyID]
	sk := configMap[models.ConfigKeyAwsSecretAccessKey]

	if ak == "" || sk == "" {
		// Fallback to Env or Shared Config
		return config.LoadDefaultConfig(ctx, config.WithRegion(region))
	}

	return config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     ak,
				SecretAccessKey: sk,
			}, nil
		})),
	)
}

// Helper: Find Debian AMI
func findDebianAMI(ctx context.Context, client *ec2.Client) (string, error) {
	// Filter for Debian 12
	out, err := client.DescribeImages(ctx, &ec2.DescribeImagesInput{
		Filters: []types.Filter{
			{Name: aws.String("name"), Values: []string{"debian-12-amd64-*"}},
			{Name: aws.String("architecture"), Values: []string{"x86_64"}},
			{Name: aws.String("virtualization-type"), Values: []string{"hvm"}},
			// Owner verified: Debian (136693071363) or AWS Marketplace
			// Using wildcard for owner or specific ID if strictly required.
			// For simplicity, we search public matched names in the account + Debian official
		},
		Owners:            []string{"136693071363"}, // Debian Official
		IncludeDeprecated: aws.Bool(false),
	})
	if err != nil {
		return "", err
	}

	if len(out.Images) == 0 {
		return "", fmt.Errorf("no Debian 12 AMI found in this region")
	}

	// Sort by CreationDate desc
	sort.Slice(out.Images, func(i, j int) bool {
		return *out.Images[i].CreationDate > *out.Images[j].CreationDate
	})

	return *out.Images[0].ImageId, nil
}

// ListRegions 获取当前账户可用的区域列表
func ListRegions(ctx context.Context) ([]string, error) {
	// Use us-east-1 as base to list regions
	cfg, err := loadAWSConfig(ctx, "us-east-1")
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	out, err := client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{
		AllRegions: aws.Bool(false), // Only list enabled regions
	})
	if err != nil {
		return nil, err
	}

	var regions []string
	for _, r := range out.Regions {
		if r.RegionName != nil {
			regions = append(regions, *r.RegionName)
		}
	}
	sort.Strings(regions)
	return regions, nil
}

type AMIInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ListFeaturedImages 获取指定区域的推荐镜像
func ListFeaturedImages(ctx context.Context, region string) ([]AMIInfo, error) {
	cfg, err := loadAWSConfig(ctx, region)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	// Search for Debian 12 and Ubuntu 22.04/24.04
	filters := []types.Filter{
		{Name: aws.String("architecture"), Values: []string{"x86_64"}},
		{Name: aws.String("virtualization-type"), Values: []string{"hvm"}},
		{Name: aws.String("state"), Values: []string{"available"}},
		{Name: aws.String("name"), Values: []string{
			"debian-12-amd64-*",
			"ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*",
			"ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-amd64-server-*",
		}},
	}

	out, err := client.DescribeImages(ctx, &ec2.DescribeImagesInput{
		Filters:           filters,
		Owners:            []string{"amazon", "136693071363", "099720109477"}, // Amazon, Debian, Canonical
		IncludeDeprecated: aws.Bool(false),
	})
	if err != nil {
		return nil, err
	}

	// Deduplicate and Sort
	// We want the LATEST of each distro.
	// Map: DistroPrefix -> Latest Image
	latestMap := make(map[string]types.Image)

	for _, img := range out.Images {
		name := *img.Name
		var key string
		if strings.HasPrefix(name, "debian-12") {
			key = "Debian 12"
		} else if strings.Contains(name, "ubuntu-jammy-22.04") {
			key = "Ubuntu 22.04 LTS"
		} else if strings.Contains(name, "ubuntu-noble-24.04") {
			key = "Ubuntu 24.04 LTS"
		} else {
			continue
		}

		if current, ok := latestMap[key]; !ok {
			latestMap[key] = img
		} else {
			// Compare CreationDate
			if *img.CreationDate > *current.CreationDate {
				latestMap[key] = img
			}
		}
	}

	var result []AMIInfo
	for k, img := range latestMap {
		desc := ""
		if img.Description != nil {
			desc = *img.Description
		}
		result = append(result, AMIInfo{
			ID:          *img.ImageId,
			Name:        k,    // Use Friendly Name
			Description: desc, // Raw desc
		})
	}

	// Sort result by Name
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}

// Helper: Ensure Security Group
func ensureSecurityGroup(ctx context.Context, client *ec2.Client) (string, error) {
	sgName := "StealthOpenSG"

	// Check exist
	res, err := client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		Filters: []types.Filter{
			{Name: aws.String("group-name"), Values: []string{sgName}},
		},
	})

	if err == nil && len(res.SecurityGroups) > 0 {
		return *res.SecurityGroups[0].GroupId, nil
	}

	// Create
	desc := "Allow all traffic for StealthForward"
	// Need VPC ID? default VPC usually used if not specified?
	// CreateSecurityGroup requires VpcId if not EC2-Classic.
	// Find default VPC
	vpcs, err := client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
		Filters: []types.Filter{{Name: aws.String("isDefault"), Values: []string{"true"}}},
	})
	if err != nil || len(vpcs.Vpcs) == 0 {
		// Try first VPC found
		vpcs, err = client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{})
		if err != nil || len(vpcs.Vpcs) == 0 {
			return "", fmt.Errorf("no VPC found")
		}
	}
	vpcID := *vpcs.Vpcs[0].VpcId

	sgRes, err := client.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(sgName),
		Description: aws.String(desc),
		VpcId:       aws.String(vpcID),
	})
	if err != nil {
		return "", err
	}
	sgID := *sgRes.GroupId

	// Add Ingress Rule (Allow All)
	_, err = client.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: aws.String(sgID),
		IpPermissions: []types.IpPermission{
			{
				IpProtocol: aws.String("-1"), // All protocols
				IpRanges: []types.IpRange{
					{CidrIp: aws.String("0.0.0.0/0")},
				},
			},
		},
	})

	return sgID, nil
}

func generateUserData(password string) string {
	if password == "" {
		password = "StealthPassword123!"
	}
	return fmt.Sprintf(`#!/bin/bash
echo "root:%s" | chpasswd
sed -i 's/^#PermitRootLogin.*/PermitRootLogin yes/' /etc/ssh/sshd_config
sed -i 's/^PermitRootLogin.*/PermitRootLogin yes/' /etc/ssh/sshd_config
sed -i 's/^PasswordAuthentication.*/PasswordAuthentication yes/' /etc/ssh/sshd_config
sed -i 's/^#PasswordAuthentication.*/PasswordAuthentication yes/' /etc/ssh/sshd_config
systemctl restart sshd
`, password)
}
