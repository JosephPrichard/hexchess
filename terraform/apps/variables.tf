variable "project" {
  description = "Project name, used as a prefix for all resources"
  type        = string
}

variable "environment" {
  description = "Environment name (e.g. uat, prod)"
  type        = string
}

variable "container_port" {
  description = "Port the container listens on"
  type        = number
}

variable "container_image" {
  description = "Docker image to run in ECS (e.g. 123456789.dkr.ecr.us-east-1.amazonaws.com/myapp)"
  type        = string
}

variable "image_tag" {
  description = "Container image SHA for this specific deployment"
  type        = string
}

variable "commit_sha" {
  description = "Commit SHA for this specific deployment"
  type        = string
}

variable "health_check_path" {
  description = "Path for the ALB health check"
  type        = string
}

variable "task_cpu" {
  description = "Fargate task CPU units (256, 512, 1024, 2048, 4096)"
  type        = number
}

variable "task_memory" {
  description = "Fargate task memory in MB"
  type        = number
}

variable "desired_count" {
  description = "Number of ECS tasks to run"
  type        = number
}
