output "region" {
  description = "AWS region"
  value       = var.region
}

output "cluster_endpoint" {
  description = "Endpoint for EKS control plane"
  value       = module.eks.cluster_endpoint
}

output "cluster_security_group_id" {
  description = "Security group ids attached to the cluster control plane"
  value       = module.eks.cluster_security_group_id
}

output "cluster_name" {
  description = "Kubernetes Cluster Name"
  value       = module.eks.cluster_name
}

output "loxilb_private_ip" {
  description = "loxilb private ip address"
  value       = module.ec2_instance.private_ip
}

output "loxilb_public_ip" {
  description = "loxilb public ip address"
  value       = module.ec2_instance.public_ip
}

output "host_private_ip" {
  description = "host private ip address"
  value       = module.ec2_instance2.private_ip
}

output "host_public_ip" {
  description = "host public ip address"
  value       = module.ec2_instance2.public_ip
}


