package cloud

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"sort"
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

	// 2. Find Debian 12 AMI
	amiID, err := findDebianAMI(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to find AMI: %v", err)
	}
	log.Printf("[Cloud] Found Debian 12 AMI: %s", amiID)

	// 3. Prepare Key Pair
	keyName := "stealth_auto_key"
	// 尝试创建 KeyPair，如果存在则忽略错误继续使用
	_, err = client.CreateKeyPair(ctx, &ec2.CreateKeyPairInput{KeyName: aws.String(keyName)})
	if err != nil {
		// Ignore if key already exists
		// In a real app we might check the error code "InvalidKeyPair.Duplicate"
		log.Printf("[Cloud] Using existing key pair: %s", keyName)
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
