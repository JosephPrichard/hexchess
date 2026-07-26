locals {
  vpcprefix = var.project

  serverlessv2_scaling_configuration = {
    min_capacity = 0.5
    max_capacity = 1.0
  }
}

module "aurora_postgresql" {
  source  = "terraform-aws-modules/rds-aurora/aws"
  version = "~> 10.0"

  name           = "${local.vpcprefix}-${var.database_cluster_name}"
  engine         = "aurora-postgresql"
  engine_version = "17.5"

  vpc_id               = var.vpc_id
  availability_zones   = var.vpc_azs
  subnets              = var.database_subnets # the subnet the database is in
  db_subnet_group_name = var.database_subnet_group_name

  enable_http_endpoint = true # enables specific queries only from authorized users (such as aws console)

  security_group_ingress_rules = { # the subnets the database is accessible from
    for idx, cidr in var.inbound_subnets_cidr_blocks :
    "private-az${idx + 1}" => {
      cidr_ipv4 = cidr
    }
  }

  engine_mode    = var.instance_class == "db.serverless" ? "provisioned" : null
  cluster_instance_class = var.instance_class
  serverlessv2_scaling_configuration = var.instance_class == "db.serverless" ? local.serverlessv2_scaling_configuration : null

  instances = {
    1 = {}
    2 = {}
  }

  master_username             = "dbadmin"
  manage_master_user_password = true

  storage_encrypted            = var.environment == "prod" ? true : false
  deletion_protection          = var.environment == "prod" ? true : false
  skip_final_snapshot          = var.environment != "prod"
  final_snapshot_identifier    = "${local.vpcprefix}-final-snapshot"
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

# Polices for Postgres
# Each of these corresponds to a database user, which must be created with the same name within AWS ADMIN CONSOLE
# An app can act as the database user by assuming the role
# To properly create ROLES and DATABASES, use the /scripts/init_database*.sql scripts

locals {
  db_users = [
    { name = "db_migrator" },
    { name = "db_readwrite" }
  ]
}

resource "aws_iam_policy" "db_role_policy_connect" {
  for_each = { for r in local.db_users : r.name => r }

  name = "${local.vpcprefix}-${var.database_cluster_name}-${each.value.name}-connect"

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
          "arn:aws:rds-db:${var.aws_region}:${var.account_id}:dbuser:${module.aurora_postgresql.cluster_resource_id}/${each.value.name}"
        ]
      }
    ]
  })
}

