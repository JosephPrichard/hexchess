module "infra" {
  source = "../global"

  project     = "hexchess"
  environment = "uat"
  aws_region  = "us-east-1"
  account_id  = "938864279852"
}