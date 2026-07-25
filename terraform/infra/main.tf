# Locals
locals {
  vpcprefix = "${var.project}-${var.environment}"
}

# VPC
module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "6.6.1"

  name = "${local.vpcprefix}-vpc"
  cidr = var.vpc_cidr

  azs              = var.vpc_azs
  public_subnets   = [for k, v in var.vpc_azs : cidrsubnet(var.vpc_cidr, 8, k)]
  private_subnets  = [for k, v in var.vpc_azs : cidrsubnet(var.vpc_cidr, 8, k + 3)]
  database_subnets = [for k, v in var.vpc_azs : cidrsubnet(var.vpc_cidr, 8, k + 6)]

  enable_nat_gateway   = true
  single_nat_gateway   = true  # cost-saving for uat env
  enable_dns_hostnames = true
}

# S3
locals {
  buckets = { for env_var_name, name in var.buckets : env_var_name => "${local.vpcprefix}-${name}" }
}

resource "aws_s3_bucket" "main" {
  for_each = local.buckets
  bucket   = each.value
}

resource "aws_s3_bucket_public_access_block" "main" {
  for_each                = local.buckets
  bucket                  = aws_s3_bucket.main[each.key].id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# Aurora Postgres
module "primarydb" {
  source = "./modules/database"

  name        = "primarydb"

  project     = var.project
  environment = var.environment
  account_id  = var.account_id
  aws_region  = var.aws_region

  vpc_id                          = module.vpc.vpc_id
  vpc_azs                         = module.vpc.azs
  vpc_private_subnets = module.vpc.private_subnets # Not publicly accessible
  vpc_private_subnets_cidr_blocks = module.vpc.private_subnets_cidr_blocks

  instance_class = "db.t4g.micro"
}

# module "metricsdb" {
#   source = "./modules/database"
#
#   name        = "metricsdb"
#   project     = var.project
#   environment = var.environment
#
#   vpc_id                          = module.vpc.vpc_id
#   vpc_azs                         = module.vpc.azs
#   vpc_private_subnets             = module.vpc.private_subnets
#   vpc_private_subnets_cidr_blocks = module.vpc.private_subnets_cidr_blocks
#
#   instance_class = "db.t4g.micro"
# }

# Redis
resource "aws_security_group" "redis_sg" {
  name        = "${local.vpcprefix}-redis-sg"
  description = "Allow Redis access from private subnets"
  vpc_id      = module.vpc.vpc_id

  ingress {
    description = "Redis access from private subnets"
    from_port   = 6379
    to_port     = 6379
    protocol    = "tcp"
    cidr_blocks = module.vpc.private_subnets_cidr_blocks
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Environment = var.environment
    Project     = var.project
  }
}

# MemoryDB
resource "aws_memorydb_subnet_group" "main" {
  name       = "${local.vpcprefix}-primarydb"
  subnet_ids = module.vpc.private_subnets # Not publicly accessible

  tags = {
    Environment = var.environment
    Project     = var.project
  }
}

resource "aws_memorydb_user" "app_user" {
  user_name     = "${local.vpcprefix}-primarydb-appuser"
  access_string = "on ~* +@all -@dangerous" # ReadWrite app permissions

  authentication_mode {
    type       = "password"
    passwords  = ["YourSecurePassword123!"] # Use a vault or variable in production
  }
}

resource "aws_memorydb_acl" "app_user_acl" {
  name       = "${local.vpcprefix}-primarydb-appuser-acl"
  user_names = [aws_memorydb_user.app_user.user_name]
}

resource "aws_memorydb_cluster" "memorydb" {
  cluster_name             = "${local.vpcprefix}-primarydb-cluster"
  acl_name                 = aws_memorydb_acl.app_user_acl.name
  node_type                = "db.t4g.small"
  engine_version           = "7.1"
  num_shards               = 3
  num_replicas_per_shard   = 1
  subnet_group_name        = aws_memorydb_subnet_group.main.name
  security_group_ids       = [aws_security_group.redis_sg.id]
  tls_enabled              = true
  snapshot_retention_limit = 7

  tags = {
    Environment = var.environment
    Project     = var.project
  }
}

# ElastiCache
resource "aws_elasticache_subnet_group" "main" {
  name       = "${local.vpcprefix}-pubsub"
  subnet_ids = module.vpc.private_subnets # Not publicly accessible

  tags = {
    Environment = var.environment
    Project     = var.project
  }
}

resource "aws_elasticache_cluster" "pubsub" {
  cluster_id        = "${local.vpcprefix}-pubsub"
  engine            = "redis"
  engine_version    = "7.1"
  node_type         = "cache.t4g.micro"
  num_cache_nodes   = 1
  port              = 6379
  subnet_group_name = aws_elasticache_subnet_group.main.name
  security_group_ids = [aws_security_group.redis_sg.id]

  snapshot_retention_limit = 0 # no persistence — pure in-memory, pub/sub only

  tags = {
    Environment = var.environment
    Project     = var.project
  }
}

# ALB
resource "aws_security_group" "alb" {
  name   = "${var.project}-${var.environment}-alb-sg"
  vpc_id = module.vpc.vpc_id

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_lb" "main" {
  name               = "${var.project}-${var.environment}-alb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = module.vpc.public_subnets # Publicly accessible
}

# Disable this once HTTPs is enabled
resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.main.arn
  port              = 80
  protocol          = "HTTP"
}

# IAM

# IAM (ecs task role)
resource "aws_iam_role" "ecs_task_execution" {
  name = "${var.project}-${var.environment}-ecs-execution"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ecs-tasks.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy_attachment" "ecs_task_execution" {
  role       = aws_iam_role.ecs_task_execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

# IAM (s3 access role)
resource "aws_iam_role" "s3_readwrite" {
  name = "${var.project}-${var.environment}-app-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ecs-tasks.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

# Allow app role to access all S3 buckets
locals {
  s3_buckets_access = [
    for _, bucket in aws_s3_bucket.main : {
      Effect   = "Allow"
      Action   = ["s3:GetObject", "s3:PutObject", "s3:DeleteObject", "s3:ListBucket"]
      Resource = [bucket.arn, "${bucket.arn}/*"]
    }
  ]
}

resource "aws_iam_role_policy" "ecs_task_s3" {
  name = "s3-access"
  role = aws_iam_role.s3_readwrite.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = local.s3_buckets_access
  })
}

# ECS Cluster
resource "aws_ecs_cluster" "main" {
  name = "${local.vpcprefix}-cluster"

  setting {
    name  = "containerInsights"
    value = "enabled"
  }
}