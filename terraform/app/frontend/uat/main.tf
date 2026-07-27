variable "commit_sha" {
  description = "Commit SHA for this specific deployment"
  type        = string
}

locals {
  infra = data.terraform_remote_state.infra.outputs
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

  vpc_id              = local.infra.vpc_id
  private_subnets_ids = local.infra.private_subnets_ids
  cluster_arn         = local.infra.cluster_arn
  alb_listener_arn    = local.infra.alb_listener_arn
  security_group_id   = local.infra.listener_security_group_id
  execution_role_arn  = local.infra.ecs_task_execution_role_arn
  task_role_arn       = local.infra.ecs_task_frontend_role_arn

  container_image = "938864279852.dkr.ecr.us-east-1.amazonaws.com/releases/hexchess/frontend"
  image_tag       = var.commit_sha

  env_vars = {
    # Replace these with route53 hostnames
    PUBLIC_APP_BASE_URL: "",
    APP_BASE_URL:        "",
  }
}