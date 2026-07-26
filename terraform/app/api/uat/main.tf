terraform {
  required_version = "1.15.8"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "6.56.0"
    }
  }

  backend "s3" {
    bucket       = "hexchess-app-api-tfstate-bucket"
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
      Project     = "hexchess-app-api"
      Environment = "uat"
      ManagedBy   = "terraform"
    }
  }
}

variable "commit_sha" {
  description = "Commit SHA for this specific deployment"
  type        = string
}

data "aws_availability_zones" "available" {
  state = "available"
}

data "aws_caller_identity" "current" {}

module "main" {
  source = "../../modules/service"

  project     = "hexchess"
  environment = "uat"
  aws_region  = "us-east-1"
  app_name    = "api"

  healthcheck_path = "/healthcheck"

  desired_task_count = 1
  task_cpu           = 256
  task_memory        = 512

  base_priority = 100
  path_patterns = ["*/api*"]

  vpc_id              = "vpc-09a4a25397617780e"
  alb_listener_arn    = "arn:aws:elasticloadbalancing:us-east-1:938864279852:listener/app/hexchess-alb/3ccbeba32ca3949b/4982eed784008b9f"
  cluster_arn         = "arn:aws:ecs:us-east-1:938864279852:cluster/hexchess-cluster"
  private_subnets_ids = ["subnet-07474f65485856165", "subnet-09119a2f7d080cccb", "subnet-0a7b59d16bf62c7bf"]
  security_group_id   = "sg-0328d917c1e1397b7"
  execution_role_arn  = "arn:aws:iam::938864279852:role/hexchess-ecs-execution-role"
  task_role_arn       = "arn:aws:iam::938864279852:role/hexchess-ecs-task-app-role"

  container_image = "938864279852.dkr.ecr.us-east-1.amazonaws.com/releases/hexchess/api"
  image_tag       = var.commit_sha

  env_vars = {
    SERVER_PORT: "8080",

    PRIMARY_DB_URL: "postgres://db_readwrite@hexchess-sor.cluster-c10fqqu5dr0g.us-east-1.rds.amazonaws.com:5432/hexchess?sslmode=require",

    REDIS_PUBSUB_NODE: "hexchess-pubsub.hl6d8h.0001.use1.cache.amazonaws.com:6379",
    REDIS_SOR_NODES:   join(",", [
      "clustercfg.hexchess-primary-cluster.hl6d8h.memorydb.us-east-1.amazonaws.com:6379",
      "hexchess-primary-cluster-0001-001.hexchess-primary-cluster.hl6d8h.memorydb.us-east-1.amazonaws.com:6379",
      "hexchess-primary-cluster-0001-002.hexchess-primary-cluster.hl6d8h.memorydb.us-east-1.amazonaws.com:6379",
      "hexchess-primary-cluster-0002-001.hexchess-primary-cluster.hl6d8h.memorydb.us-east-1.amazonaws.com:6379",
      "hexchess-primary-cluster-0002-002.hexchess-primary-cluster.hl6d8h.memorydb.us-east-1.amazonaws.com:6379"
    ]),

    PROFILE_BUCKET_NAME: "hexchess-profiles",

    # Replace this with Route53 hostname
    ALLOWED_ORIGINS: "http://localhost:5173",

    ACTIVE_PROFILE: "test"

    # OTEL_EXPORTER_OTLP_ENDPOINT: "localhost:3100"
  }
}