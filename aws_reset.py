import boto3
import os

def reset_and_save_key():
    ak = os.environ.get('AWS_ACCESS_KEY_ID')
    sk = os.environ.get('AWS_SECRET_ACCESS_KEY')
    region = "ap-northeast-1" 
    
    ec2 = boto3.resource('ec2', region_name=region, aws_access_key_id=ak, aws_secret_access_key=sk)
    client = boto3.client('ec2', region_name=region, aws_access_key_id=ak, aws_secret_access_key=sk)
    
    print(f"[{region}] 正在清理旧实例...")
    # 查找所有我们创建的机器
    filters = [{'Name': 'instance-state-name', 'Values': ['running', 'pending', 'stopped']}]
    instances = list(ec2.instances.filter(Filters=filters))
    
    my_instances = []
    for i in instances:
        # 只删 t3.medium 或者特定 Key 的，防止误删（虽然你是空号）
        if i.instance_type == 't3.medium': 
            my_instances.append(i.id)
            
    if my_instances:
        print(f"正在终止: {my_instances}")
        ec2.instances.filter(InstanceIds=my_instances).terminate()
        print("等待终止完成（稍候自动继续）...")
        # 简单等待一下，不用 wait_until_terminated 为了省时间，只要 Key 释放就行
    else:
        print("无旧实例需要清理。")

    # 重点：处理密钥
    key_name = 'stealth_final_key'
    pem_file = 'stealth_final_key.pem' 
    
    # 先尝试删除旧 Key (如果不删，create 会报错且拿不到私钥)
    try:
        print(f"重置密钥对: {key_name}")
        client.delete_key_pair(KeyName=key_name)
    except Exception:
        pass
        
    # 创建新 Key 并保存
    key_pair = client.create_key_pair(KeyName=key_name)
    private_key = key_pair['KeyMaterial']
    
    with open(pem_file, 'w') as f:
        f.write(private_key)
    
    print(f"\n>>>>>> 成功创建新密钥! 文件已保存在当前目录: {os.path.abspath(pem_file)} <<<<<<\n")

    # 再次创建实例
    print(f"[{region}] 正在创建新实例...")
    
    # 查找 AMI
    ami_response = client.describe_images(
        Filters=[{'Name': 'name', 'Values': ['ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*']}],
        Owners=['099720109477']
    )
    images = sorted(ami_response['Images'], key=lambda x: x['CreationDate'], reverse=True)
    ami_id = images[0]['ImageId']
    
    # 安全组
    sg_name = 'StealthFINAL'
    sg_id = None
    try:
        # 尝试创建
        vpcs = list(ec2.vpcs.all())
        sg = ec2.create_security_group(GroupName=sg_name, Description='Final', VpcId=vpcs[0].id)
        sg.authorize_ingress(IpProtocol='-1', CidrIp='0.0.0.0/0')
        sg_id = sg.id
    except Exception:
        # 已存在则查找
        sgs = client.describe_security_groups(Filters=[{'Name': 'group-name', 'Values': [sg_name]}])
        sg_id = sgs['SecurityGroups'][0]['GroupId']

    # User Data (双重保险：改 root 密码 + 改 ubuntu 密码)
    user_data = '''#!/bin/bash
# 1. 改 Root 密码
echo "root:Stealth123!@#" | chpasswd
# 2. 改 Ubuntu 用户密码 (防止只能用 ubuntu 登录)
echo "ubuntu:Stealth123!@#" | chpasswd
# 3. 允许密码登录
sed -i 's/^#PermitRootLogin.*/PermitRootLogin yes/' /etc/ssh/sshd_config
sed -i 's/^PermitRootLogin.*/PermitRootLogin yes/' /etc/ssh/sshd_config
sed -i 's/^PasswordAuthentication.*/PasswordAuthentication yes/' /etc/ssh/sshd_config
sed -i 's/^#PasswordAuthentication.*/PasswordAuthentication yes/' /etc/ssh/sshd_config
systemctl restart sshd
'''

    instances = ec2.create_instances(
        ImageId=ami_id,
        MinCount=1, MaxCount=1,
        InstanceType='t3.medium',
        KeyName=key_name,
        SecurityGroupIds=[sg_id],
        UserData=user_data
    )
    
    inst = instances[0]
    print(f"实例已创建: {inst.id}")
    print("等待 IP...")
    inst.wait_until_running()
    inst.reload()
    
    print(f"\n==========================================")
    print(f"新 IP: {inst.public_ip_address}")
    print(f"私钥文件: {pem_file} (请务必用这个文件登录)")
    print(f"用户名: ubuntu (如果 root 不行)")
    print(f"密码: Stealth123!@#")
    print(f"==========================================\n")

if __name__ == "__main__":
    reset_and_save_key()
