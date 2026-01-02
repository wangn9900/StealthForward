package cloud

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lightsail"
	"github.com/aws/aws-sdk-go-v2/service/lightsail/types"
)

// LightsailBundle 代表光帆套餐
type LightsailBundle struct {
	ID       string  `json:"id"`
	Price    float32 `json:"price"`
	RamSize  float32 `json:"ram_size"` // in GB
	CpuCount int32   `json:"cpu_count"`
	DiskSize int32   `json:"disk_size"` // in GB
	Transfer float32 `json:"transfer"`  // in TB usually, API might return GB
	Name     string  `json:"name"`
}

// LightsailBlueprint 代表光帆系统镜像
type LightsailBlueprint struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Group       string `json:"group"`
}

// GetLightsailClient 创建客户端
func GetLightsailClient(ctx context.Context, region string) (*lightsail.Client, error) {
	cfg, err := loadAWSConfig(ctx, region)
	if err != nil {
		return nil, err
	}
	return lightsail.NewFromConfig(cfg), nil
}

// ListLightsailRegions 获取光帆可用区域
func ListLightsailRegions(ctx context.Context) ([]string, error) {
	// Always use us-east-1 to query regions
	client, err := GetLightsailClient(ctx, "us-east-1")
	if err != nil {
		return nil, err
	}

	out, err := client.GetRegions(ctx, &lightsail.GetRegionsInput{
		IncludeAvailabilityZones: aws.Bool(false),
	})
	if err != nil {
		return nil, err
	}

	var regions []string
	for _, r := range out.Regions {
		if r.Name != nil {
			regions = append(regions, string(*r.Name))
		}
	}
	sort.Strings(regions)
	return regions, nil
}

// ListLightsailBundles 获取套餐列表 (只显示 Linux)
func ListLightsailBundles(ctx context.Context, region string) ([]LightsailBundle, error) {
	client, err := GetLightsailClient(ctx, region)
	if err != nil {
		return nil, err
	}

	out, err := client.GetBundles(ctx, &lightsail.GetBundlesInput{
		IncludeInactive: aws.Bool(false),
	})
	if err != nil {
		return nil, err
	}

	var bundles []LightsailBundle
	for _, b := range out.Bundles {
		// Filter for Linux (Applies mostly to Blueprints, but Bundles can be platform specific implicitly sometimes, mostly generic)
		// Lightsail API returns all bundles.
		// We usually want standard bundles, not Windows only if distinguished.
		// supportedPlatforms field in Bundle allows filtering.
		isLinux := false
		for _, p := range b.SupportedPlatforms {
			if p == types.InstancePlatformLinuxUnix {
				isLinux = true
				break
			}
		}
		if !isLinux {
			continue
		}

		bundles = append(bundles, LightsailBundle{
			ID:       *b.BundleId,
			Price:    *b.Price,
			RamSize:  *b.RamSizeInGb,
			CpuCount: *b.CpuCount,
			DiskSize: *b.DiskSizeInGb,
			Transfer: *b.TransferPerMonthInGb, // actually GB
			Name:     *b.Name,
		})
	}

	// Sort by Price
	sort.Slice(bundles, func(i, j int) bool {
		return bundles[i].Price < bundles[j].Price
	})

	return bundles, nil
}

// ListLightsailBlueprints 获取系统镜像 (OS Only)
func ListLightsailBlueprints(ctx context.Context, region string) ([]LightsailBlueprint, error) {
	client, err := GetLightsailClient(ctx, region)
	if err != nil {
		return nil, err
	}

	out, err := client.GetBlueprints(ctx, &lightsail.GetBlueprintsInput{
		IncludeInactive: aws.Bool(false),
	})
	if err != nil {
		return nil, err
	}

	var blueprints []LightsailBlueprint
	for _, bp := range out.Blueprints {
		// Only Linux/Unix
		if bp.Platform != types.InstancePlatformLinuxUnix {
			continue
		}
		// Only OS, avoid Apps (WordPress etc)
		// Blueprint type: os or app
		if bp.Type != types.BlueprintTypeOs {
			continue
		}

		blueprints = append(blueprints, LightsailBlueprint{
			ID:          *bp.BlueprintId,
			Name:        *bp.Name,
			Description: aws.ToString(bp.Description),
			Group:       aws.ToString(bp.Group),
		})
	}

	// Sort: Debian preferred, then Ubuntu, etc
	sort.Slice(blueprints, func(i, j int) bool {
		// custom sort logic could go here
		return blueprints[i].Name < blueprints[j].Name
	})

	return blueprints, nil
}

type CreateLightsailRequest struct {
	Region       string `json:"region"`
	BundleID     string `json:"bundle_id"`     // e.g. micro_3_0
	BlueprintID  string `json:"blueprint_id"`  // e.g. debian_12
	InstanceName string `json:"instance_name"` // optional
	RootPassword string `json:"root_password"`
}

type CreateLightsailResponse struct {
	InstanceName string `json:"instance_name"`
	PublicIP     string `json:"public_ip"`
}

// ProvisionLightsailInstance 创建光帆实例
func ProvisionLightsailInstance(ctx context.Context, req CreateLightsailRequest) (*CreateLightsailResponse, error) {
	client, err := GetLightsailClient(ctx, req.Region)
	if err != nil {
		return nil, err
	}

	// 1. Name
	name := req.InstanceName
	if name == "" {
		name = fmt.Sprintf("Stealth-LS-%s-%d", req.Region, time.Now().Unix()%10000)
	}

	// 2. Availability Zone (pick first)
	regionsOut, _ := client.GetRegions(ctx, &lightsail.GetRegionsInput{IncludeAvailabilityZones: aws.Bool(true)})
	var azName string
	for _, r := range regionsOut.Regions {
		if string(*r.Name) == req.Region {
			if len(r.AvailabilityZones) > 0 {
				azName = *r.AvailabilityZones[0].ZoneName
			}
			break
		}
	}
	if azName == "" {
		// Fallback, might error if req.Region is invalid, but let API handle
		azName = req.Region + "a"
	}

	// 3. KeyPair (Optional, we rely on UserData password)
	// Lightsail needs a keypair usually or uses default. We won't block on keypair.

	// 4. UserData
	userData := generateUserData(req.RootPassword)
	// Lightsail expects plain text or base64? RunInstances is base64. CreateInstances is string.
	// SDK docs say: Only one of 'userData' or 'keyPairName' is required? No, usually optional.
	// We will pass user data script. It effectively enables root login.
	// Lightsail userData is passed as string.

	// 5. Create Instance
	_, err = client.CreateInstances(ctx, &lightsail.CreateInstancesInput{
		InstanceNames:    []string{name},
		AvailabilityZone: aws.String(azName),
		BlueprintId:      aws.String(req.BlueprintID),
		BundleId:         aws.String(req.BundleID),
		UserData:         aws.String(userData),
		// KeyPairName: aws.String("default"), // Optional
	})
	if err != nil {
		return nil, fmt.Errorf("CreateInstances failed: %v", err)
	}
	log.Printf("[Cloud-LS] Instance creating: %s", name)

	// 6. Wait for Available (to attach Static IP and Open Ports)
	// Lightsail operations are async. We need to poll instance state.
	// It usually takes 20-40 seconds to become 'running' or ready for networking ops.

	// Open Ports Loop (try until success, max attempts)
	// We need to allow TCP/UDP all ports. Lightsail allows range 0-65535.
	go func() {
		// We run this in background or hold connection?
		// User wants "Perfect", better to wait and return IP.
	}()

	// Wait loop
	maxRetries := 60
	ip := ""
	for i := 0; i < maxRetries; i++ {
		time.Sleep(3 * time.Second)
		inst, err := client.GetInstance(ctx, &lightsail.GetInstanceInput{InstanceName: aws.String(name)})
		if err == nil && inst.Instance != nil {
			state := ""
			if inst.Instance.State != nil {
				state = *inst.Instance.State.Name
			}
			if state == "running" {
				ip = aws.ToString(inst.Instance.PublicIpAddress)
				break
			}
		}
	}

	if ip == "" {
		return nil, fmt.Errorf("timeout waiting for instance running")
	}

	// 7. Attach Static IP (The "Perfect" touch)
	staticIpName := name + "-StaticIP"
	_, err = client.AllocateStaticIp(ctx, &lightsail.AllocateStaticIpInput{
		StaticIpName: aws.String(staticIpName),
	})
	if err == nil {
		_, err = client.AttachStaticIp(ctx, &lightsail.AttachStaticIpInput{
			InstanceName: aws.String(name),
			StaticIpName: aws.String(staticIpName),
		})
		if err != nil {
			log.Printf("[Cloud-LS] Failed to attach static IP: %v", err)
		} else {
			// Get new IP
			staticIpRes, _ := client.GetStaticIp(ctx, &lightsail.GetStaticIpInput{StaticIpName: aws.String(staticIpName)})
			if staticIpRes != nil && staticIpRes.StaticIp != nil {
				ip = *staticIpRes.StaticIp.IpAddress
			}
		}
	} else {
		log.Printf("[Cloud-LS] Failed to allocate static IP: %v", err)
	}

	// 8. Open Ports (Firewall)
	// Allowed: tcp/0-65535, udp/0-65535
	// Note: PutInstancePublicPorts replaces ALL existing rules.
	_, err = client.PutInstancePublicPorts(ctx, &lightsail.PutInstancePublicPortsInput{
		InstanceName: aws.String(name),
		PortInfos: []types.PortInfo{
			{FromPort: aws.Int32(0), ToPort: aws.Int32(65535), Protocol: types.NetworkProtocolTcp},
			{FromPort: aws.Int32(0), ToPort: aws.Int32(65535), Protocol: types.NetworkProtocolUdp},
			// Ping (ICMP) is implicit usually or types.NetworkProtocolIcmp?
			// V2 SDK types.NetworkProtocolIcmp exists?
			// types.NetworkProtocolAll? No. Just TCP/UDP covers 99%.
		},
	})
	if err != nil {
		log.Printf("[Cloud-LS] Failed to open ports: %v", err)
	}

	return &CreateLightsailResponse{
		InstanceName: name,
		PublicIP:     ip,
	}, nil
}

// TerminateLightsailInstance 销毁光帆及静态IP
func TerminateLightsailInstance(ctx context.Context, region, instanceName string) error {
	client, err := GetLightsailClient(ctx, region)
	if err != nil {
		return err
	}

	// 1. Check if Static IP attached
	inst, err := client.GetInstance(ctx, &lightsail.GetInstanceInput{InstanceName: aws.String(instanceName)})
	var staticIpName string
	if err == nil && inst.Instance != nil && inst.Instance.PublicIpAddress != nil {
		// If it is a static IP, verify.
		// Actually, simpler way: GetStaticIp(name + "-StaticIP")
		// We used naming convention name + "-StaticIP"
		potentialName := instanceName + "-StaticIP"
		_, err := client.GetStaticIp(ctx, &lightsail.GetStaticIpInput{StaticIpName: aws.String(potentialName)})
		if err == nil {
			staticIpName = potentialName
		}
	}

	// 2. Delete Instance
	_, err = client.DeleteInstance(ctx, &lightsail.DeleteInstanceInput{
		InstanceName: aws.String(instanceName),
	})
	if err != nil {
		return err
	}

	// 3. Delete Static IP (Release)
	if staticIpName != "" {
		// Wait a bit? Usually static IP release requires detachment which happens on instance terminate?
		// We might need to wait for instance to be shutting-down/terminated.
		// We'll try async or best effort.
		go func() {
			time.Sleep(5 * time.Second)
			client.ReleaseStaticIp(ctx, &lightsail.ReleaseStaticIpInput{StaticIpName: aws.String(staticIpName)})
		}()
	}

	return nil
}

// RotateLightsailIP 更换光帆实例的静态 IP
func RotateLightsailIP(ctx context.Context, region, instanceName string) (string, error) {
	client, err := GetLightsailClient(ctx, region)
	if err != nil {
		return "", err
	}

	// 1. Find Attached Static IP
	inst, err := client.GetInstance(ctx, &lightsail.GetInstanceInput{InstanceName: aws.String(instanceName)})
	if err != nil {
		return "", fmt.Errorf("instance not found: %v", err)
	}

	oldStaticIpName := ""
	if inst.Instance != nil && inst.Instance.PublicIpAddress != nil {
		// Try to guess default name first
		potentialName := instanceName + "-StaticIP"
		res, err := client.GetStaticIp(ctx, &lightsail.GetStaticIpInput{StaticIpName: aws.String(potentialName)})
		if err == nil && res.StaticIp != nil && *res.StaticIp.IpAddress == *inst.Instance.PublicIpAddress {
			oldStaticIpName = potentialName
		} else {
			// If name mismatch, we need to iterate all static IPs to find which one is attached to this instance
			// Not efficient but Lightsail API doesn't list attachments directly on instance object detail deeply sometimes
			// Actually GetStaticIps lists attachments.
			allIps, _ := client.GetStaticIps(ctx, &lightsail.GetStaticIpsInput{})
			for _, sip := range allIps.StaticIps {
				if sip.AttachedTo != nil && *sip.AttachedTo == instanceName {
					oldStaticIpName = *sip.Name
					break
				}
			}
		}
	}

	// 2. Detach & Release Old
	if oldStaticIpName != "" {
		log.Printf("[Cloud-LS] Detaching old IP: %s", oldStaticIpName)
		_, err := client.DetachStaticIp(ctx, &lightsail.DetachStaticIpInput{StaticIpName: aws.String(oldStaticIpName)})
		if err != nil {
			return "", fmt.Errorf("failed to detach old ip: %v", err)
		}

		// Wait for detach?
		time.Sleep(2 * time.Second)

		_, err = client.ReleaseStaticIp(ctx, &lightsail.ReleaseStaticIpInput{StaticIpName: aws.String(oldStaticIpName)})
		if err != nil {
			log.Printf("[Cloud-LS] Warning: failed to release old ip: %v", err)
		}
	} else {
		log.Printf("[Cloud-LS] No static IP found attached, allocating new one directly.")
	}

	// 3. Allocate New
	// Randomize name slightly to avoid rapid release/allocate conflict
	newStaticIpName := fmt.Sprintf("%s-StaticIP-%d", instanceName, time.Now().Unix()%1000)

	log.Printf("[Cloud-LS] Allocating new IP: %s", newStaticIpName)
	allocRes, err := client.AllocateStaticIp(ctx, &lightsail.AllocateStaticIpInput{StaticIpName: aws.String(newStaticIpName)})
	if err != nil {
		return "", fmt.Errorf("failed to allocate new ip: %v", err)
	}

	// 4. Attach New
	_, err = client.AttachStaticIp(ctx, &lightsail.AttachStaticIpInput{
		InstanceName: aws.String(instanceName),
		StaticIpName: aws.String(newStaticIpName),
	})
	if err != nil {
		// Rollback?
		client.ReleaseStaticIp(ctx, &lightsail.ReleaseStaticIpInput{StaticIpName: aws.String(newStaticIpName)})
		return "", fmt.Errorf("failed to attach new ip: %v", err)
	}

	// Return IP
	if allocRes.StaticIp != nil {
		return *allocRes.StaticIp.IpAddress, nil
	}
	return "", nil
}
