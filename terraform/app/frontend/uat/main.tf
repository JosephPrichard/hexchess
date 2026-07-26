terraform {
  required_version = "1.15.8"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "6.56.0"
    }
  }

  backend "s3" {
    bucket       = "hexchess-app-frontend-tfstate-bucket"
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
      Project     = "hexchess-app-frontend"
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
  app_name    = "frontend"

  desired_task_count = 1
  task_cpu           = 256
  task_memory        = 512

  base_priority = 200

  vpc_id              = "vpc-09a4a25397617780e"
  alb_listener_arn    = "arn:aws:elasticloadbalancing:us-east-1:938864279852:listener/app/hexchess-alb/3ccbeba32ca3949b/4982eed784008b9f"
  cluster_arn         = "arn:aws:ecs:us-east-1:938864279852:cluster/hexchess-cluster"
  private_subnets_ids = ["subnet-07474f65485856165", "subnet-09119a2f7d080cccb", "subnet-0a7b59d16bf62c7bf"]
  security_group_id   = "sg-0328d917c1e1397b7"
  execution_role_arn  = "arn:aws:iam::938864279852:role/hexchess-ecs-execution-role"
  task_role_arn       = "arn:aws:iam::938864279852:role/hexchess-ecs-frontend-role"

  container_image = "938864279852.dkr.ecr.us-east-1.amazonaws.com/releases/hexchess/frontend"
  image_tag       = var.commit_sha

  env_vars = {
    # Replace these with route53 hostnames
    PUBLIC_APP_BASE_URL: "",
    APP_BASE_URL:        "",
  }
}