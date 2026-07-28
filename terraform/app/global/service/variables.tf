# Project / Environment Globals
variable "project" {
  description = "Project name, used as a prefix for all resources"
  type        = string
}

variable "environment" {
  description = "Environment name (e.g. uat, prod)"
  type        = string
}

variable "aws_region" {
  description = "AWS region to deploy into"
  type        = string
}

# ALB only configurations (can be left default if ALB is not enabled)
variable "alb_listener_arn" {
  description = "The ARN of the ALB listener any ALB listener rules will be attached to"
  type        = string
  default     = null
}

variable "base_priority" {
  description = "Base priority of any ALB listener rules. Requires an open range of (base_priority, base_priority+100]"
  type        = number
  default     = 0
}

variable "path_patterns" {
  description = "The path patterns any ALB listener rules will match"
  type        = list(string)
  default     = ["*"]
}

variable "vpc_id" {
  description = "VPC id the ALB target group will be in. Required if service will contain an ALB listener rule"
  type        = string
  default     = null
}

variable "healthcheck_path" {
  description = "Health check path ALB target group will use to ping the app"
  type        = string
  default     = "/healthcheck"
}

variable "rollout" {
  description = "Rollout kind of app deployment (e.g blue, green, switch)"
  type        = string
  default     = "blue"
}

variable "green_switch_weight" {
  description = "The % of traffic to be directed to the green app when rollout is set to 'switch'"
  type        = string
  default     = 0
}

variable "app_port" {
  description = "Port the ALB will route traffic to"
  type        = number
  default     = 8080
}

# Deployment specific values
variable "app_name" {
  description = "Name of the app to be deployed (must be unique)"
  type        = string
}
variable "task_cpu" {
  description = "Number of CPU units each ecs task is allocated (256, etc.)"
  type        = number
}

variable "task_memory" {
  description = "Memory in MB each ecs task is allocated"
  type        = number
}

variable "desired_task_count" {
  description = "Number of ECS tasks"
  type        = number
}

# References to infra
variable "cluster_arn" {
  description = "ARN of the ECS cluster the service belongs to"
  type        = string
}

variable "private_subnets_ids" {
  description = "Private subnet ids of the VPN the ECS cluster is in"
  type        = list(string)
}

variable "security_group_id" {
  description = "Security group of the ECS service"
  type        = string
}

variable "execution_role_arn" {
  description = "Task execution role of the ECS task"
  type        = string
}

variable "task_role_arn" {
  description = "App role of the ECS task"
  type        = string
}

# Task specific parameters
variable "env_vars" {
  description = "Environment variables for the ecs task definition"
  type        = map(string)
}

variable "container_image" {
  description = "Docker image to run in ECS (e.g. 123456789.dkr.ecr.us-east-1.amazonaws.com/myapp)"
  type        = string
}

variable "image_tag" {
  description = "Container image SHA for this specific deployment"
  type        = string
}