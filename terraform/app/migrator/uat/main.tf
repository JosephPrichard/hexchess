terraform {
  required_version = "1.15.8"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "6.56.0"
    }
  }

  backend "s3" {
    bucket       = "hexchess-app-migrator-tfstate-bucket"
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
      Project     = "hexchess-app-migrator"
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

module "main" {
  source = "../../modules/service"

  project     = "hexchess"
  environment = "uat"
  aws_region  = "us-east-1"
  app_name    = "migrator"

  desired_task_count = 1
  task_cpu           = 256
  task_memory        = 512

  cluster_arn        = "arn:aws:ecs:us-east-1:938864279852:cluster/hexchess-cluster"
  private_subnets_ids = ["subnet-07474f65485856165", "subnet-09119a2f7d080cccb", "subnet-0a7b59d16bf62c7bf"]
  security_group_id  = "sg-0c8d93cf644bde6d8"
  execution_role_arn = "arn:aws:iam::938864279852:role/hexchess-ecs-execution-role"
  task_role_arn      = "arn:aws:iam::938864279852:role/hexchess-ecs-task-migrator-role"

  container_image = "938864279852.dkr.ecr.us-east-1.amazonaws.com/releases/hexchess/migrator"
  image_tag       = var.commit_sha

  env_vars = {
    PRIMARY_DB_URL: "postgres://db_migrator@hexchess-sor.cluster-c10fqqu5dr0g.us-east-1.rds.amazonaws.com:5432/hexchess?sslmode=require",
    METRICS_DB_URL: "postgres://db_migrator@hexchess-sor.cluster-c10fqqu5dr0g.us-east-1.rds.amazonaws.com:5432/metrics?sslmode=require"

    ACTIVE_PROFILE: "test"
  }
}