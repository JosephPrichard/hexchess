data "aws_caller_identity" "current" {}

locals {
  prefix = var.project
}

### NETWORKING CONFIGS

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

# DNS

data "aws_route53_zone" "main" {
  name         = var.hosted_zone_name
  private_zone = false
}

# Each environment gets a "green" subdomain for allowing green deployments
resource "aws_route53_record" "cname" {
  zone_id = data.aws_route53_zone.main.zone_id
  name    = "green.${var.domain_name}"
  type    = "CNAME"
  ttl     = 3600
  # A green subdomain is actually just pointed to the main domain
  records = [var.domain_name]
}

# Point the environment's DNS record to the public LB
resource "aws_route53_record" "alb_alias" {
  zone_id = data.aws_route53_zone.main.zone_id
  name    = var.domain_name
  type    = "A"

  alias {
    name                   = aws_lb.public.dns_name
    zone_id                = aws_lb.public.zone_id
    evaluate_target_health = true
  }
}

### RDS DATABASES

locals {
  database_cluster_name = "${local.prefix}-sor"

  serverlessv2_scaling_configuration = {
    min_capacity = 0.5
    max_capacity = 1.0
  }
}

# Aurora Postgres SOR DB
module "sor-db" {
  source  = "terraform-aws-modules/rds-aurora/aws"
  version = "~> 10.0"

  name           = local.database_cluster_name
  engine         = "aurora-postgresql"
  engine_version = "17.5"

  vpc_id               = module.vpc.vpc_id
  availability_zones   = module.vpc.azs
  subnets              = module.vpc.database_subnets # the subnet the database is in
  db_subnet_group_name = module.vpc.database_subnet_group_name

  enable_http_endpoint = true # enables specific queries only from authorized users (such as aws console)

  # Database is only accessible from the private subnet
  security_group_ingress_rules = {
    for idx, cidr in module.vpc.private_subnets_cidr_blocks :
    "private-az${idx + 1}" => {
      # Automatically creates the correct security group for port 5432
      cidr_ipv4 = cidr
    }
  }

  engine_mode    = var.postgres_instance_type == "db.serverless" ? "provisioned" : null
  cluster_instance_class = var.postgres_instance_type
  serverlessv2_scaling_configuration = var.postgres_instance_type == "db.serverless" ? local.serverlessv2_scaling_configuration : null

  instances = {
    1 = {}
    2 = {}
  }

  master_username             = "dbadmin"
  manage_master_user_password = true

  storage_encrypted            = var.environment == "prod" ? true : false
  deletion_protection          = var.environment == "prod" ? true : false
  skip_final_snapshot          = var.environment != "prod"
  final_snapshot_identifier    = "${local.prefix}-final-snapshot"
  backup_retention_period      = var.environment == "prod" ? 30 : 1
  preferred_backup_window      = "03:00-04:00"
  preferred_maintenance_window = "sun:04:30-sun:05:30"
  apply_immediately            = var.environment != "prod"
  copy_tags_to_snapshot        = true

  iam_database_authentication_enabled = true

  enabled_cloudwatch_logs_exports = ["postgresql"]

  tags = {
    Environment = var.environment
    Project     = var.project
  }
}

locals {
  policy_arn_map = {
    for key, policy in aws_iam_policy.db_role_policy_connect : key => policy.arn
  }
}

# Polices for Postgres
# Each of these corresponds to a database user, which must be created with the same name within AWS ADMIN CONSOLE
# An app can act as the database user by assuming the role
# To properly create ROLES and DATABASES, use the ./init/init_*.sql scripts

locals {
  db_users = [
    { name = "db_readwrite" }
  ]
}

resource "aws_iam_policy" "db_role_policy_connect" {
  for_each = { for r in local.db_users : r.name => r }

  name = "${local.prefix}-${local.database_cluster_name}-${each.value.name}-connect"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "AuroraIamDbAuth"
        Effect   = "Allow"
        # Allows for CONNECT, it is USER the role is linked to in the database that allows for access permissions
        Action   = ["rds-db:connect"]
        Resource = [
          # ${each.value.name} is the USER name in the database for this policy
          "arn:aws:rds-db:${var.aws_region}:${var.account_id}:dbuser:${module.sor-db.cluster_resource_id}/${each.value.name}"
        ]
      }
    ]
  })
}

### BLOCK STORAGE

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

### REDIS

# MemoryDB
resource "aws_memorydb_cluster" "memorydb" {
  name                     = "${local.prefix}-primary-cluster"
  # MemoryDB is not secured with a password for the meantime, we rely on firewall rules for security
  acl_name                 = "open-access"
  node_type                = var.memorydb_node_type
  engine_version           = "7.2"
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

### APP INFRA

# ECS Cluster
resource "aws_ecs_cluster" "main" {
  name = "${local.prefix}-cluster"

  setting {
    name  = "containerInsights"
    value = "enabled"
  }
}

### LOAD BALANCERS

# ALB
# The primary function of the ALB is to provide routing rules for traffic coming from the NLB AND within the same private subnet

resource "aws_lb" "private" {
  name               = "${local.prefix}-alb-private"
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]

  # Not internet facing
  internal = true
  subnets  = module.vpc.private_subnets
}

locals {
  alb_listeners = {
    http = {
      port            = 80,
      protocol        = "HTTP",
      certificate_arn = null,
      ssl_policy      = null
    },
    https = {
      port            = 443,
      protocol        = "HTTPS",
      certificate_arn = var.certificate_arn,
      ssl_policy      = "ELBSecurityPolicy-TLS13-1-2-2021-06"
    },
  }
}

resource "aws_lb_listener" "alb_listeners" {
  for_each = local.alb_listeners

  load_balancer_arn = aws_lb.private.arn
  port              = each.value.port
  protocol          = each.value.protocol
  certificate_arn   = each.value.certificate_arn
  ssl_policy        = each.value.ssl_policy

  default_action {
    type = "fixed-response"

    # Default response is 404 because applications will be adding their rules to this ALB on a case by case basis
    fixed_response {
      content_type = "text/plain"
      message_body = "Application is missing or moved"
      status_code  = "404"
    }
  }
}

# Public NLB
# The primary function of this NLB is to take traffic from the internet and route it to the private ALB

locals {
  nlb_listeners = {
    http = {
      port = 80,
      protocol = "HTTP",
    },
    https = {
      port = 443
      protocol = "HTTPS",
    },
  }
}

resource "aws_lb" "public" {
  name               = "${local.prefix}-nlb-public"
  load_balancer_type = "network"
  security_groups    = [aws_security_group.alb.id]

  enable_cross_zone_load_balancing = true

  # Internet facing
  internal = false
  subnets  = module.vpc.public_subnets
}

resource "aws_lb_listener" "public" {
  for_each = local.nlb_listeners

  load_balancer_arn = aws_lb.public.arn
  port              = each.value.port
  protocol          = "TCP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.nlb_to_alb_tg[each.key].arn
  }
}

resource "aws_lb_target_group" "nlb_to_alb_tg" {
  for_each = local.nlb_listeners

  name        = "${local.prefix}-nlb-to-alb-tg-${each.key}"
  vpc_id      = module.vpc.vpc_id
  target_type = "alb"

  port     = each.value.port
  protocol = "TCP"

  health_check {
    protocol            = each.value.protocol
    # We hit the application or ALB provided healthcheck
    path                = "/healthcheck"
    port                = each.value.port
    healthy_threshold   = 3
    unhealthy_threshold = 3
    interval            = 30
    # If the private ALB can send any kind of response at all, we can route traffic to it. This includes the private ALB's default 404 response.
    matcher             = "200-499"
  }
}

resource "aws_lb_target_group_attachment" "nlb_to_alb_tg_attach" {
  for_each = local.nlb_listeners

  target_group_arn = aws_lb_target_group.nlb_to_alb_tg[each.key].arn
  target_id        = aws_lb.private.arn
  port             = each.value.port
}

### SECURITY GROUPS

# Redis Security Group
resource "aws_security_group" "redis_sg" {
  name        = "${local.prefix}-redis-sg"
  description = "Allow Redis access from private subnets"
  vpc_id      = module.vpc.vpc_id

  ingress {
    description = "Redis access from private subnets"
    from_port   = 6379
    to_port     = 6379
    protocol    = "tcp"
    # Allow all traffic from any application in the private subnet
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

# LB Security Group
resource "aws_security_group" "alb" {
  # This is a security group designed for private ALB specifically, but it is reused by the NLB

  name        = "${local.prefix}-alb-sg"
  description = "Allow inbound HTTP/HTTPs traffic for ALB"
  vpc_id      = module.vpc.vpc_id

  # ALBs permit traffic from ALL IPs, since ALBs are designed to support user facing APIs
  # However whether or not the ALB will be accessible from the internet is decided by which subnet the ALB itself is in

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
    security_groups = [aws_security_group.alb.id]  # only from load balancer
  }

  ingress {
    from_port       = 5173
    to_port         = 5173
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]  # only from load balancer
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

  # This security groups is used for background workers that need to access infrastructure but provide no user facing APIs

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

### IAM POLICIES

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

### IAM ROLES

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
  policy_arn = local.policy_arn_map["db_readwrite"]
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
  policy_arn = local.policy_arn_map["db_readwrite"]
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