provider "aws" {
  region = var.region
}

data "aws_availability_zones" "available" {}

locals {
  cluster_name = "loxilb-eks-${random_string.suffix.result}"
}

resource "random_string" "suffix" {
  length  = 8
  special = false
}

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "5.0.0"
  name = "loxilb-cicd-vpc"  
  cidr = "10.0.0.0/16"
  azs  = slice(data.aws_availability_zones.available.names, 0, 3)

  private_subnets = ["10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"]
  public_subnets  = ["10.0.4.0/24", "10.0.5.0/24", "10.0.6.0/24"]

  enable_nat_gateway   = true
  single_nat_gateway   = true
  enable_dns_hostnames = true

  public_subnet_tags = {
    "kubernetes.io/cluster/${local.cluster_name}" = "shared"
    "kubernetes.io/role/elb"                      = 1
  }

  private_subnet_tags = {
    "kubernetes.io/cluster/${local.cluster_name}" = "shared"
    "kubernetes.io/role/internal-elb"             = 1
  }
}

module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "19.15.3"

  cluster_name    = local.cluster_name
  cluster_version = "1.27"

  vpc_id                         = module.vpc.vpc_id
  subnet_ids                     = module.vpc.private_subnets
  cluster_endpoint_public_access = true

  eks_managed_node_group_defaults = {
    ami_type = "AL2_x86_64"
  }
  
  eks_managed_node_groups = {
    node = {
      name = "node-group"
      instance_types = ["t3.small"]
      min_size     = 1
      max_size     = 3
      desired_size = 2
    }
  }
}

module "ec2_instance" {
  source  = "terraform-aws-modules/ec2-instance/aws"
  name = "loxilb_node"
  
  ami = "ami-0c016e131d2da56e8"
  instance_type          = "t3.small"
  key_name               = "aws-osaka"
  monitoring             = true
  
  vpc_security_group_ids = [ module.security-group.security_group_id]
  subnet_id              = module.vpc.public_subnets[0]
  associate_public_ip_address = "true"

  tags = {
    Terraform   = "true"
    Environment = "dev"
  }
}    

module "ec2_instance2" {
  source  = "terraform-aws-modules/ec2-instance/aws"
  name = "host_node"

  ami = "ami-0d126351255167386"
  instance_type          = "t3.small"
  key_name               = "aws-osaka"
  monitoring             = true

  vpc_security_group_ids = [ module.security-group.security_group_id]
  subnet_id              = module.vpc.public_subnets[0]
  associate_public_ip_address = "true"

  tags = {
    Terraform   = "true"
    Environment = "dev"
  }
}


# https://aws.amazon.com/blogs/containers/amazon-ebs-csi-driver-is-now-generally-available-in-amazon-eks-add-ons/ 
data "aws_iam_policy" "ebs_csi_policy" {
  arn = "arn:aws:iam::aws:policy/service-role/AmazonEBSCSIDriverPolicy"
}

module "irsa-ebs-csi" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-assumable-role-with-oidc"
  version = "4.7.0"

  create_role                   = true
  role_name                     = "AmazonEKSTFEBSCSIRole-${module.eks.cluster_name}"
  provider_url                  = module.eks.oidc_provider
  role_policy_arns              = [data.aws_iam_policy.ebs_csi_policy.arn]
  oidc_fully_qualified_subjects = ["system:serviceaccount:kube-system:ebs-csi-controller-sa"]
}

module "security-group" {
  source  = "terraform-aws-modules/security-group/aws" 
  version = "5.1.0"                                    
  name        = "LoxiLB-node-sg" 
  description = "LoxiLB node security group"
  vpc_id      = module.vpc.vpc_id
  use_name_prefix = "false" 
  ingress_with_cidr_blocks = [
    {
      from_port   = 22
      to_port     = 22
      protocol    = "tcp"
      description = "ssh"
      cidr_blocks = "0.0.0.0/0"
    },{
      from_port   = 80
      to_port     = 80
      protocol    = "tcp"
      description = "http"
      cidr_blocks = "0.0.0.0/0"
    },{
      from_port   = 8080
      to_port     = 8080
      protocol    = "tcp"
      description = "http_tmp"
      cidr_blocks = "0.0.0.0/0"
    },{
      from_port   = 11111
      to_port     = 11111
      protocol    = "tcp"
      description = "loxilb"
      cidr_blocks = "0.0.0.0/0"
    },{
      from_port   = 0
      to_port     = 65535
      protocol    = 132
      description = "sctp protocol"
      cidr_blocks = "0.0.0.0/0"
    },{
      from_port   = 50003
      to_port     = 50003
      protocol    = "udp"
      description = "udp"
      cidr_blocks = "0.0.0.0/0"
    },
  ]
  ingress_with_source_security_group_id = [
    {
      from_port   = 0
      to_port     = 65535
      protocol    = "all"
      description = "all"
      source_security_group_id = module.eks.node_security_group_id
    },
  ]
  egress_with_cidr_blocks = [
    {
      from_port   = 0
      to_port     = 0
      protocol    = "-1"
      description = "all"
      cidr_blocks = "0.0.0.0/0"
    }
]
}

resource "aws_security_group_rule" "ingress_with_cluter_to_loxilb" {
       description              = "all"
       from_port                = 1
       protocol                 = "all"
       security_group_id        = module.eks.node_security_group_id
       source_security_group_id = module.security-group.security_group_id
       to_port                  = 65535
       type                     = "ingress"
}

