data "aws_caller_identity" "current" {}

# Locals
locals {
  prefix = var.project
}

# VPC
module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "6.6.1"

  name = "${local.prefix}-vpc"
  cidr = var.vpc_cidr

  azs              = var.vpc_azs
  public_subnets   = [for k, v in var.vpc_azs : cidrsubnet(var.vpc_cidr, 8, k)]
  private_subnets  = [for k, v in var.vpc_azs : cidrsubnet(var.vpc_cidr, 8, k + 3)]
  database_subnets = [for k, v in var.vpc_azs : cidrsubnet(var.vpc_cidr, 8, k + 6)]

  enable_nat_gateway   = true
  single_nat_gateway   = true  # cost-saving for uat env
  enable_dns_hostnames = true
}

# Aurora Postgres
module "sor-db" {
  source = "./modules/database"

  database_cluster_name = "sor" # system of record

  project     = var.project
  environment = var.environment
  account_id  = var.account_id
  aws_region  = var.aws_region

  vpc_id                      = module.vpc.vpc_id
  vpc_azs                     = module.vpc.azs
  database_subnets            = module.vpc.database_subnets # Not publicly accessible
  database_subnet_group_name  = module.vpc.database_subnet_group_name
  inbound_subnets_cidr_blocks = module.vpc.private_subnets_cidr_blocks

  instance_class = var.postgres_instance_type
}

# S3 Buckets
locals {
  buckets = { for env_var_name, name in var.buckets : env_var_name => "${local.prefix}-${name}" }
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

# Redis
resource "aws_security_group" "redis_sg" {
  name        = "${local.prefix}-redis-sg"
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
resource "aws_memorydb_cluster" "memorydb" {
  name                     = "${local.prefix}-primary-cluster"
  # MemoryDB is not secured with a password for the meantime, we rely on network ACLs for security
  acl_name                 = "open-access"
  node_type                = var.memorydb_node_type
  engine_version           = "7.1"
  num_shards               = var.memorydb_shards
  num_replicas_per_shard   = 1
  subnet_group_name        = aws_memorydb_subnet_group.main.name
  security_group_ids       = [aws_security_group.redis_sg.id]
  tls_enabled              = true
  snapshot_retention_limit = 1

  tags = {
    Environment = var.environment
    Project     = var.project
  }
}

resource "aws_memorydb_subnet_group" "main" {
  name       = "${local.prefix}-primary"
  subnet_ids = module.vpc.private_subnets # Not publicly accessible

  tags = {
    Environment = var.environment
    Project     = var.project
  }
}

# ElastiCache
resource "aws_elasticache_cluster" "pubsub" {
  cluster_id         = "${local.prefix}-pubsub"
  engine             = "redis"
  engine_version     = "7.1"
  node_type          = var.elasticache_node_type
  num_cache_nodes    = 1
  port               = 6379
  subnet_group_name  = aws_elasticache_subnet_group.main.name
  security_group_ids = [aws_security_group.redis_sg.id]

  snapshot_retention_limit = 0 # no persistence — pure in-memory, pub/sub only

  tags = {
    Environment = var.environment
    Project     = var.project
  }
}

resource "aws_elasticache_subnet_group" "main" {
  name       = "${local.prefix}-pubsub"
  subnet_ids = module.vpc.private_subnets # Not publicly accessible

  tags = {
    Environment = var.environment
    Project     = var.project
  }
}

# ECS Cluster
resource "aws_ecs_cluster" "main" {
  name = "${local.prefix}-cluster"

  setting {
    name  = "containerInsights"
    value = "enabled"
  }
}

# ALB
resource "aws_lb" "main" {
  name               = "${local.prefix}-alb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = module.vpc.public_subnets # Publicly accessible
}

resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.main.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type = "fixed-response"

    fixed_response {
      content_type = "text/plain"
      message_body = "Cannot match load balancer rule"
      status_code  = "404"
    }
  }
}

resource "aws_security_group" "alb" {
  name        = "${local.prefix}-alb-sg"
  description = "Allow inbound HTTP/HTTPs traffic for ALB"
  vpc_id      = module.vpc.vpc_id

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

# ECS Service Security Group
resource "aws_security_group" "ecs_service_listener" {
  name        = "${local.prefix}-ecs-service-listener-sg"
  description = "Allows inbound TCP traffic on app listener ports"
  vpc_id      = module.vpc.vpc_id

  # Apps always listen on 8080 (standard backend port) or 5173 (default vite port for frontend)
  ingress {
    from_port       = 8080
    to_port         = 8080
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]  # only from ALB
  }

  ingress {
    from_port       = 5173
    to_port         = 5173
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]  # only from ALB
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "ecs_service_worker" {
  name        = "${local.prefix}-ecs-service-worker-sg"
  description = "Allows all egress network traffic"
  vpc_id      = module.vpc.vpc_id

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

# IAM (S3 Bucket Access Policy)
resource "aws_iam_policy" "s3_readwrite_policy" {
  name = "${local.prefix}-s3-readwrite"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      for _, bucket in aws_s3_bucket.main : {
        Effect   = "Allow"
        Action   = ["s3:GetObject", "s3:PutObject", "s3:DeleteObject", "s3:ListBucket"]
        Resource = [bucket.arn, "${bucket.arn}/*"]
      }
    ]
  })
}

# IAM (ECS Cluster Task Execution Role)
resource "aws_iam_role" "ecs_task_execution_role" {
  name = "${local.prefix}-ecs-execution-role"

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
  role       = aws_iam_role.ecs_task_execution_role.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

# IAM (ECS Cluster Task App Role)
resource "aws_iam_role" "ecs_task_app_role" {
  name = "${local.prefix}-ecs-task-app-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ecs-tasks.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy_attachment" "ecs_task_app_role_s3_access" {
  role       = aws_iam_role.ecs_task_app_role.name
  policy_arn = aws_iam_policy.s3_readwrite_policy.arn
}

resource "aws_iam_role_policy_attachment" "ecs_task_app_role_rds_access" {
  role       = aws_iam_role.ecs_task_app_role.name
  policy_arn = module.sor-db.policy_arn_map["db_readwrite"]
}

# IAM (ECS Cluster Task Migrator Role)
resource "aws_iam_role" "ecs_task_migrator_role" {
  name = "${local.prefix}-ecs-task-migrator-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ecs-tasks.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy_attachment" "ecs_task_migrator_role_rds_access" {
  role       = aws_iam_role.ecs_task_migrator_role.name
  policy_arn = module.sor-db.policy_arn_map["db_migrator"]
}

# IAM (ECS Cluster Task Frontend Role)
resource "aws_iam_role" "ecs_task_frontend_role" {
  name = "${local.prefix}-ecs-frontend-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ecs-tasks.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}