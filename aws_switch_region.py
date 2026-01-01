import boto3
import os

def cleanup_us_and_try_asia():
    ak = os.environ.get('AWS_ACCESS_KEY_ID')
    sk = os.environ.get('AWS_SECRET_ACCESS_KEY')
    
    # 1. 清理美国机器
    us_region = "us-east-1"
    print(f"[{us_region}] 正在连接以清理实例...")
    ec2_us = boto3.resource('ec2', region_name=us_region, aws_access_key_id=ak, aws_secret_access_key=sk)
    
    # 查找所有运行中的机器（刚才创建的）
    instances = ec2_us.instances.filter(
        Filters=[
            {'Name': 'instance-state-name', 'Values': ['running', 'pending']},
            {'Name': 'key-name', 'Values': ['stealth_auto_key']} 
        ]
    )
    
    ids_to_terminate = [i.id for i in instances]
    if ids_to_terminate:
        print(f"[{us_region}] 发现实例 {ids_to_terminate}，正在终止...")
        ec2_us.instances.filter(InstanceIds=ids_to_terminate).terminate()
        print(f"[{us_region}] 终止指令已发送。")
    else:
        print(f"[{us_region}] 未发现相关实例。")

    # 2. 尝试日本区域 (Tokyo)
    target_region = "ap-northeast-1" 
    print(f"\n[{target_region}] 正在尝试创建新实例 (日本-东京)...")
    
    ec2_target = boto3.resource('ec2', region_name=target_region, aws_access_key_id=ak, aws_secret_access_key=sk)
    ec2_target_client = boto3.client('ec2', region_name=target_region, aws_access_key_id=ak, aws_secret_access_key=sk)

    try:
        # 搜索 AMI (Ubuntu 22.04) - 日本区
        print(f"[{target_region}] 搜索镜像...")
        ami_response = ec2_target_client.describe_images(
            Filters=[{'Name': 'name', 'Values': ['ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*']}],
            Owners=['099720109477']
        )
        images = sorted(ami_response['Images'], key=lambda x: x['CreationDate'], reverse=True)
        if not images:
            print("错误: 未找到镜像。")
            return
        ami_id = images[0]['ImageId']

        # 密钥对
        key_name = 'stealth_jp_key'
        try:
            ec2_target_client.create_key_pair(KeyName=key_name)
            print(f"[{target_region}] 创建密钥对: {key_name}")
        except Exception:
            pass # 已存在

        # 安全组
        sg_id = None
        try:
            vpcs = list(ec2_target.vpcs.all())
            if vpcs:
                sg_name = 'StealthOpenSG'
                try:
                    sg = ec2_target.create_security_group(GroupName=sg_name, Description='All Open', VpcId=vpcs[0].id)
                    sg.authorize_ingress(IpProtocol='-1', CidrIp='0.0.0.0/0')
                    sg_id = sg.id
                except Exception:
                    sgs = ec2_target_client.describe_security_groups(Filters=[{'Name': 'group-name', 'Values': [sg_name]}])
                    sg_id = sgs['SecurityGroups'][0]['GroupId']
        except Exception as e:
            print(f"安全组错误: {e}")
            pass # 尝试继续

        # User Data
        user_data = '''#!/bin/bash
echo "root:Stealth123!@#" | chpasswd
sed -i 's/^#PermitRootLogin.*/PermitRootLogin yes/' /etc/ssh/sshd_config
sed -i 's/^PermitRootLogin.*/PermitRootLogin yes/' /etc/ssh/sshd_config
sed -i 's/^PasswordAuthentication.*/PasswordAuthentication yes/' /etc/ssh/sshd_config
sed -i 's/^#PasswordAuthentication.*/PasswordAuthentication yes/' /etc/ssh/sshd_config
systemctl restart sshd
'''

        print(f"[{target_region}] 启动实例 (t3.medium)...")
        instances = ec2_target.create_instances(
            ImageId=ami_id,MinCount=1,MaxCount=1,
            InstanceType='t3.medium',KeyName=key_name,
            SecurityGroupIds=[sg_id] if sg_id else [],
            UserData=user_data
        )
        new_vm = instances[0]
        print(f"[{target_region}] 等待 IP 分配...")
        new_vm.wait_until_running()
        new_vm.reload()
        
        print(f"\n>>> 成功! 日本机器 IP: {new_vm.public_ip_address}")
        print(f">>> 密码: Stealth123!@#")

    except Exception as e:
        print(f"[{target_region}] 创建失败: {e}")
        # 如果日本也不行，尝试新加坡
        print("建议尝试新加坡 (ap-southeast-1) ...")

if __name__ == "__main__":
    cleanup_us_and_try_asia()
