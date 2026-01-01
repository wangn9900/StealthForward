package cloud

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/cloudflare/cloudflare-go"
	"github.com/wangn9900/StealthForward/internal/database"
	"github.com/wangn9900/StealthForward/internal/models"
)

// RotateIPForInstance 核心业务逻辑：换 EC2 IP 并更新 Cloudflare DNS
// 返回新 IP，或者错误
func RotateIPForInstance(ctx context.Context, region, instanceID, zoneName, recordName string) (string, error) {
	// 0. 读取 AWS 配置 (DB -> Env)
	var settings []models.SystemSetting
	configMap := make(map[string]string)
	if err := database.DB.Find(&settings).Error; err == nil {
		for _, s := range settings {
			configMap[s.Key] = s.Value
		}
	}

	ak := configMap[models.ConfigKeyAwsAccessKeyID]
	if ak == "" {
		ak = os.Getenv("AWS_ACCESS_KEY_ID")
	}
	sk := configMap[models.ConfigKeyAwsSecretAccessKey]
	if sk == "" {
		sk = os.Getenv("AWS_SECRET_ACCESS_KEY")
	}

	// 1. 初始化 AWS
	var cfg aws.Config
	var err error

	if ak != "" && sk != "" {
		// 使用静态凭证
		cfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(region),
			config.WithCredentialsProvider(aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
				return aws.Credentials{
					AccessKeyID:     ak,
					SecretAccessKey: sk,
				}, nil
			})),
		)
	} else {
		// 回退到默认链
		cfg, err = config.LoadDefaultConfig(ctx, config.WithRegion(region))
	}

	if err != nil {
		return "", fmt.Errorf("aws load config error: %v", err)
	}
	ec2Client := ec2.NewFromConfig(cfg)

	// 2. 申请新 EIP
	allocRes, err := ec2Client.AllocateAddress(ctx, &ec2.AllocateAddressInput{
		Domain: types.DomainTypeVpc,
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeElasticIp,
				Tags: []types.Tag{
					{Key: aws.String("Name"), Value: aws.String("AutoRotated-" + recordName)},
					{Key: aws.String("CreatedBy"), Value: aws.String("StealthController")},
				},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("aws allocate address error: %v", err)
	}
	newAllocationID := *allocRes.AllocationId
	newPublicIP := *allocRes.PublicIp
	log.Printf("[Cloud] Allocated new EIP: %s (%s)", newPublicIP, newAllocationID)

	// 3. 查找实例当前绑定的 EIP (以便后续释放)
	// 使用 DescribeAddresses 查找绑定到该实例的 EIP
	addrRes, err := ec2Client.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("instance-id"),
				Values: []string{instanceID},
			},
		},
	})
	var oldAllocationID string

	if err == nil && len(addrRes.Addresses) > 0 {
		if addrRes.Addresses[0].AllocationId != nil {
			oldAllocationID = *addrRes.Addresses[0].AllocationId
		}
	}

	// 4. 绑定新 EIP 到实例 (强制覆盖 Reassociation)
	_, err = ec2Client.AssociateAddress(ctx, &ec2.AssociateAddressInput{
		InstanceId:         aws.String(instanceID),
		AllocationId:       aws.String(newAllocationID),
		AllowReassociation: aws.Bool(true),
	})
	if err != nil {
		// 绑定失败，回滚：释放新申请的 IP
		ec2Client.ReleaseAddress(ctx, &ec2.ReleaseAddressInput{AllocationId: aws.String(newAllocationID)})
		return "", fmt.Errorf("aws associate address error: %v", err)
	}
	log.Printf("[Cloud] Associated new EIP %s to instance %s", newPublicIP, instanceID)

	// 5. 释放旧 EIP (如果存在)
	if oldAllocationID != "" {
		// 等待解绑完成后释放？通常 AssociateAddress 成功意味着旧的已经解绑。
		// 但为了保险，可以启动一个 goroutine 延迟释放，或者直接释放
		// ReleaseAddress 只有在 Address 没有被 Associate 时才能成功？
		// 对于覆盖绑定，旧 Association 会自动解除，但 Address 还是 Allocated 的。
		_, err := ec2Client.ReleaseAddress(ctx, &ec2.ReleaseAddressInput{
			AllocationId: aws.String(oldAllocationID),
		})
		if err != nil {
			log.Printf("[Cloud] Warning: Failed to release old EIP %s: %v", oldAllocationID, err)
		} else {
			log.Printf("[Cloud] Released old EIP: %s", oldAllocationID)
		}
	} else {
		log.Printf("[Cloud] Review: No old EIP found to release. Was using standard public IP?")
	}

	// 6. 更新 Cloudflare DNS
	if zoneName != "" && recordName != "" {
		err = updateCloudflareDNS(ctx, zoneName, recordName, newPublicIP)
		if err != nil {
			log.Printf("[Cloud] Error updating DNS (but IP rotated): %v", err)
			return newPublicIP, fmt.Errorf("ip rotated to %s but dns update failed: %v", newPublicIP, err)
		}
		log.Printf("[Cloud] Updated Cloudflare DNS %s.%s -> %s", recordName, zoneName, newPublicIP)
	}

	return newPublicIP, nil
}

func updateCloudflareDNS(ctx context.Context, zoneName, recordName, newIP string) error {
	// 读取 CF Token
	var setting models.SystemSetting
	var apiToken string
	if err := database.DB.Where("key = ?", models.ConfigKeyCfApiToken).First(&setting).Error; err == nil {
		apiToken = setting.Value
	}
	if apiToken == "" {
		apiToken = os.Getenv("CF_API_TOKEN")
	}

	if apiToken == "" {
		return fmt.Errorf("CF_API_TOKEN environment variable not set (or not in db)")
	}

	api, err := cloudflare.NewWithAPIToken(apiToken)
	if err != nil {
		return err
	}

	// 获取 Zone ID
	zoneID, err := api.ZoneIDByName(zoneName)
	if err != nil {
		return fmt.Errorf("cf get zone id error: %v", err)
	}

	// 查找 A 记录
	// 不同的库版本 ListDNSRecords 参数可能不同，这里尝试通用写法
	// recordName 通常是子域名，比如 "transitnodeawjp"
	// Full name = transitnodeawjp.2233006.xyz
	fullRecordName := recordName
	if !strings.HasSuffix(recordName, zoneName) {
		fullRecordName = fmt.Sprintf("%s.%s", recordName, zoneName)
	}

	stats, _, err := api.ListDNSRecords(ctx, cloudflare.ZoneIdentifier(zoneID), cloudflare.ListDNSRecordsParams{
		Type: "A",
		Name: fullRecordName,
	})
	if err != nil {
		return fmt.Errorf("cf list records error: %v", err)
	}

	if len(stats) == 0 {
		return fmt.Errorf("dns record %s not found", fullRecordName)
	}

	recordID := stats[0].ID

	// 更新记录
	_, err = api.UpdateDNSRecord(ctx, cloudflare.ZoneIdentifier(zoneID), cloudflare.UpdateDNSRecordParams{
		ID:      recordID,
		Content: newIP,
		Type:    "A",
		Name:    fullRecordName,
		Proxied: stats[0].Proxied, // 保持原有的 Proxy 状态 (通常为 false)
		TTL:     stats[0].TTL,
		Comment: aws.String("Auto rotated by StealthController at " + time.Now().Format(time.RFC3339)),
	})

	return err
}
