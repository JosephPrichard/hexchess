variable "project" {
  description = "Project name, used as a prefix for all resources"
  type        = string
  default     = "hexchess"
}

variable "environment" {
  description = "Environment name (e.g. uat, prod)"
  type        = string
}

variable "aws_region" {
  description = "AWS region to deploy into"
  type        = string
}

variable "account_id" {
  description = "AWS account id"
  type        = string
}

variable "vpc_cidr" {
  description = "CIDR block for the VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "vpc_azs" {
  description = "AZs for the VPC"
  type        = list(string)
  default     = ["us-east-1a", "us-east-1b", "us-east-1c"] # This *must* return at least 3 AZs
}

variable "buckets" {
  description = "Names of S3 buckets to create"
  type        = map(string)
  default     = {
    PROFILE_BUCKET = "profiles"
  }
}