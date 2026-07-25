locals {
  vpcprefix = "${var.project}-${var.environment}"
}

module "aurora_postgresql" {
  source  = "terraform-aws-modules/rds-aurora/aws"
  version = "~> 10.0"

  name           = "${local.vpcprefix}-${var.name}"
  engine         = "aurora-postgresql"
  engine_version = "17.5"

  vpc_id               = var.vpc_id
  availability_zones   = var.vpc_azs
  subnets              = var.vpc_private_subnets # database is accessible from private subnet only

  security_group_ingress_rules = {
    private-az1 = {
      cidr_ipv4 = var.vpc_private_subnets_cidr_blocks[0]
    }
    private-az2 = {
      cidr_ipv4 = var.vpc_private_subnets_cidr_blocks[1]
    }
    private-az3 = {
      cidr_ipv4 = var.vpc_private_subnets_cidr_blocks[2]
    }
  }

  instances = {
    1 = {
      instance_class          = var.instance_class
      db_parameter_group_name = "default.aurora-postgresql17"
    }
    2 = {
      instance_class          = var.instance_class
      db_parameter_group_name = "default.aurora-postgresql17"
    }
    3 = {
      instance_class          = var.instance_class
      db_parameter_group_name = "default.aurora-postgresql17"
    }
  }

  master_username             = "dbadmin"
  manage_master_user_password = true

  storage_encrypted            = true
  deletion_protection          = var.environment == "prod" ? true : false
  skip_final_snapshot          = var.environment != "prod"
  final_snapshot_identifier    = "${local.vpcprefix}-final-snapshot"
  backup_retention_period      = var.environment == "prod" ? 30 : 0
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

# Access roles for Postgres
# Each of these corresponds to a database user, which must be created with the same name within AWS ADMIN CONSOLE
# An app access the database user by assuming the role
locals {
  db_roles = [
    {
      name           = "db_migrator"
      assume_service = "ecs-tasks.amazonaws.com"
    },
    {
      name           = "db_readwrite"
      assume_service = "ecs-tasks.amazonaws.com"
    }
  ]
}

resource "aws_iam_role" "db_role" {
  for_each = { for r in local.db_roles : r.name => r }

  name = "${local.vpcprefix}-${each.value.name}"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Service = each.value.assume_service
        }
        Action = "sts:AssumeRole"
      }
    ]
  })

  tags = {
    Environment = var.environment
    Project     = var.project
  }
}

resource "aws_iam_role_policy" "db_role_connect" {
  for_each = { for r in local.db_roles : r.name => r }

  name = "${local.vpcprefix}-${each.value.name}-connect"
  role = aws_iam_role.db_role[each.key].id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "AuroraIamDbAuth"
        Effect   = "Allow"
        Action   = ["rds-db:connect"]
        Resource = [
          "arn:aws:rds-db:${var.aws_region}:${var.account_id}:dbuser:${module.aurora_postgresql.cluster_resource_id}/${each.value.name}"
        ]
      }
    ]
  })
}