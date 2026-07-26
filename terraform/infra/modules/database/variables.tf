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

variable "database_cluster_name" {
  description = "Database name"
  type        = string
}

variable "vpc_id" {
  description = "VPC the database and all clients are in"
  type        = string
}

variable "vpc_azs" {
  description = "AZs the database and all clients are in"
  type        = list(string)
}

variable "database_subnets" {
  description = "Subnets the database is in (generally the vpc database subnets)"
  type        = list(string)
}

variable "database_subnet_group_name" {
  description = "Subnet group name of database_subnets"
  type        = string
}

variable "inbound_subnets_cidr_blocks" {
  description = "The subnets the database will be accessible from (the subnet the clients are in)"
  type        = list(string)
}

variable "instance_class" {
  description = "Performance class of the database (e.g serverless)"
  type        = string
}
