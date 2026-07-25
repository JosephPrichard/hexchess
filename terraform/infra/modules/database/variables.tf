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

variable "account_id" {
  description = "AWS account id"
  type        = string
}

variable "name" {
  description = "Database name"
  type        = string
}

variable "vpc_id" {
  type = string
}

variable "vpc_azs" {
  type = string
}

variable "vpc_private_subnets" {
  type = string
}

variable "vpc_private_subnets_cidr_blocks" {
  type = list(string)
}

variable "instance_class" {
  type = string
}
