terraform {
  required_version = "1.15.8"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "6.56.0"
    }
  }

  backend "s3" {
    bucket       = "hexchess-app-cronjob-tfstate-bucket"
    key          = "terraform.tfstate"
    region       = "us-east-1"
    use_lockfile = true
    encrypt      = true
  }
}

provider "aws" {
  region = "us-east-1"

  default_tags {
    tags = {
      Project     = "hexchess-app-cronjob"
      Environment = "uat"
      ManagedBy   = "terraform"
    }
  }
}

data "aws_availability_zones" "available" {
  state = "available"
}

data "aws_caller_identity" "current" {}

variable "commit_sha" {
  description = "Commit SHA for this specific deployment"
  type        = string
}

# module "jobs" {
#   for_each = {
#     1 = { job = "sync-leaderboard" },
#     2 = { job = "clear-s3-orphans" }
#   }
#
#   source     = "../../modules/lambda"
#
#   project    = "hexchess"
#   aws_region = "us-east-1"
#   environment = "uat"
#   lambda_name  = "cronjob"
#
#   schedule_expression = "cron(0 3 ? * MON *)"
#   memory_size         = 128
#   timeout             = 900 # 15 minutes
#
#   lambda_role_arn = "arn:aws:iam::938864279852:role/hexchess-lambda-exec-role"
#
#   container_image = "releases/hexchess/migrator"
#   image_tag     = var.commit_sha
#
#   env_vars = {
#     PRIMARY_DB_URL: "postgres://db_readwrite@hexchess-sor.cluster-c10fqqu5dr0g.us-east-1.rds.amazonaws.com:5432/hexchess",
#
#     REDIS_SOR_NODES:   join(",", [
#       "clustercfg.hexchess-primary-cluster.hl6d8h.memorydb.us-east-1.amazonaws.com:6379",
#       "hexchess-primary-cluster-0001-001.hexchess-primary-cluster.hl6d8h.memorydb.us-east-1.amazonaws.com:6379",
#       "hexchess-primary-cluster-0001-002.hexchess-primary-cluster.hl6d8h.memorydb.us-east-1.amazonaws.com:6379",
#       "hexchess-primary-cluster-0002-001.hexchess-primary-cluster.hl6d8h.memorydb.us-east-1.amazonaws.com:6379",
#       "hexchess-primary-cluster-0002-002.hexchess-primary-cluster.hl6d8h.memorydb.us-east-1.amazonaws.com:6379"
#     ]),
#
#     JOB_NAME: each.value.job,
#     ACTIVE_PROFILE: "test"
#   }
# }