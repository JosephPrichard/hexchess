module "infra" {
  source = "../global"

  project     = "hexchess"
  environment = "uat"
  aws_region  = "us-east-1"
  account_id  = "938864279852"

  hosted_zone_name = "hexagonchess.app"
  domain_name      = "uat.hexagonchess.app"
  certificate_arn  = "arn:aws:acm:us-east-1:938864279852:certificate/d308e249-a4d7-4b9d-9b2b-85526cbfd0d3"
}