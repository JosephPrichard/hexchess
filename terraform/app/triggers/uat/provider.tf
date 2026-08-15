terraform {
  required_version = "1.15.8"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "6.56.0"
    }
  }

  backend "s3" {
    bucket       = "hexchess-app-triggers-tfstate-bucket"
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
      Project     = "hexchess-app-triggers"
      Environment = "uat"
      ManagedBy   = "terraform"
    }
  }
}

data "terraform_remote_state" "infra" {
  backend = "s3"
  config = {
    bucket = "hexchess-infra-tfstate-bucket"
    key    = "terraform.tfstate"
    region = "us-east-1"
  }
}

data "aws_availability_zones" "available" {
  state = "available"
}

data "aws_caller_identity" "current" {}