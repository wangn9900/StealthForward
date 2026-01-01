import boto3
import os
import time

def create_instance():
    # 1. 验证环境变量
    ak = os.environ.get('AWS_ACCESS_KEY_ID')
    sk = os.environ.get('AWS_SECRET_ACCESS_KEY')
    
    if not ak or not sk:
        print("错误: 未检测到环境变量 AWS_ACCESS_KEY_ID 或 AWS_SECRET_ACCESS_KEY")
        print("请先在 PowerShell 中执行: $env:AWS_ACCESS_KEY_ID='...' 和 $env:AWS_SECRET_ACCESS_KEY='...'")
        return

    region = os.environ.get('AWS_DEFAULT_REGION', 'ap-east-1') # 优先读取环境变量，默认香港
    print(f"正在连接 AWS 区域: {region} ...")

    ec2 = boto3.resource('ec2', region_name=region, aws_access_key_id=ak, aws_secret_access_key=sk)
    ec2_client = boto3.client('ec2', region_name=region, aws_access_key_id=ak, aws_secret_access_key=sk)

    try:
        # 2. 检查基本权限 (列出实例)
        print("验证凭证有效性...")
        ec2_client.describe_instances(MaxResults=5)
        print("凭证有效！")
    except Exception as e:
        print(f"连接失败: {str(e)}")
        return

    # 3. 参数配置
    # Debian 12 (通过 AWS CLI 查找最新的 Debian 12 AMI ID，这里预设一个香港的常用 ID，或者动态查找)
    # 为了稳妥，我们动态搜索官方 Debian 12 AMI
    print("搜索最新的 Debian 12 AMI...")
    ami_response = ec2_client.describe_images(
        Filters=[
            {'Name': 'name', 'Values': ['debian-12-amd64-*']},
            {'Name': 'architecture', 'Values': ['x86_64']},
            {'Name': 'owner-alias', 'Values': ['aws-marketplace']}, # 有时是 amazon 或 136693071363 (Debian org)
            # 更稳妥的是直接用 Debian 官方 Account ID: 136693071363
        ],
        Owners=['136693071363'], # Debian 官方
        IncludeDeprecated=False
    )
    
    # 排序取最新的
    images = sorted(ami_response['Images'], key=lambda x: x['CreationDate'], reverse=True)
    if not images:
        print("未找到 Debian 12 AMI，尝试 Ubuntu 22.04...")
        # Fallback to Ubuntu
        ami_response = ec2_client.describe_images(
            Filters=[
                {'Name': 'name', 'Values': ['ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*']},
            ],
            Owners=['099720109477'] # Canonical
        )
        images = sorted(ami_response['Images'], key=lambda x: x['CreationDate'], reverse=True)
    
    if not images:
        print("错误: 无法找到合适的 AMI 镜像。")
        return

    ami_id = images[0]['ImageId']
    ami_name = images[0]['Name']
    print(f"使用镜像: {ami_name} ({ami_id})")

    # 实例类型: t3.medium (2vCPU, 4GB RAM)
    instance_type = 't3.medium' 
    print(f"实例规格: {instance_type} (2vCPU, 4GB RAM)")

    # 4. 创建密钥对 (如果不存在)
    key_name = 'stealth_auto_key'
    try:
        key_pair = ec2_client.create_key_pair(KeyName=key_name)
        pem_content = key_pair['KeyMaterial']
        with open('stealth_auto_key.pem', 'w') as f:
            f.write(pem_content)
        print("已创建新密钥对: stealth_auto_key.pem (请妥善保存)")
    except ec2_client.exceptions.KeyPairInfo: # 若已存在
        print(f"使用现有密钥对: {key_name}")

    # 5. 创建安全组 (开放所有端口，方便测试)
    sg_name = 'StealthOpenSG'
    sg_id = None
    try:
        vpcs = list(ec2.vpcs.all())
        if not vpcs:
            print("错误: 该区域没有默认 VPC。")
            return
        vpc_id = vpcs[0].id
        
        try:
            sg = ec2.create_security_group(GroupName=sg_name, Description='Allow all traffic', VpcId=vpc_id)
            sg_id = sg.id
            sg.authorize_ingress(IpProtocol='-1', CidrIp='0.0.0.0/0') # 开放所有端口
            print(f"已创建安全组: {sg_name} ({sg_id})")
        except Exception: # 可能已存在
            sgs = ec2_client.describe_security_groups(Filters=[{'Name': 'group-name', 'Values': [sg_name]}])
            sg_id = sgs['SecurityGroups'][0]['GroupId']
            print(f"使用现有安全组: {sg_name} ({sg_id})")
            
    except Exception as e:
        print(f"安全组配置出错: {e}")
        return

    # User Data (启动脚本: 设置 root 密码并允许登录)
    # 注意：AWS 默认禁止 root 密码登录，必须修改 sshd_config
    user_data = '''#!/bin/bash
echo "root:Stealth123!@#" | chpasswd
sed -i 's/^#PermitRootLogin.*/PermitRootLogin yes/' /etc/ssh/sshd_config
sed -i 's/^PermitRootLogin.*/PermitRootLogin yes/' /etc/ssh/sshd_config
sed -i 's/^PasswordAuthentication.*/PasswordAuthentication yes/' /etc/ssh/sshd_config
sed -i 's/^#PasswordAuthentication.*/PasswordAuthentication yes/' /etc/ssh/sshd_config
systemctl restart sshd
'''

    # 6. 启动实例
    print("正在启动实例...")
    instances = ec2.create_instances(
        ImageId=ami_id,
        MinCount=1,
        MaxCount=1,
        InstanceType=instance_type,
        KeyName=key_name,
        SecurityGroupIds=[sg_id],
        UserData=user_data,
        TagSpecifications=[{
            'ResourceType': 'instance',
            'Tags': [{'Key': 'Name', 'Value': 'Stealth-HK-Node'}]
        }]
    )
    
    instance = instances[0]
    print(f"实例已创建! ID: {instance.id}")
    print("等待实例运行并分配 IP...")
    
    instance.wait_until_running()
    instance.reload()
    
    public_ip = instance.public_ip_address
    print(f"\n==========================================")
    print(f"成功启动! 实例信息如下:")
    print(f"IP 地址: {public_ip}")
    print(f"区域: 香港 (ap-east-1)")
    print(f"配置: {instance_type} (2C4G)")
    print(f"初始 root 密码: Stealth123!@#")
    print(f"SSH 连接: ssh root@{public_ip}")
    print(f"==========================================\n")
    print("注意: 建议登录后立即修改密码！")

if __name__ == "__main__":
    create_instance()
