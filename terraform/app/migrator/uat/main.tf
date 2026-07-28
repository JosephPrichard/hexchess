variable "commit_sha" {
  description = "Commit SHA for this specific deployment"
  type        = string
}

locals {
  infra = data.terraform_remote_state.infra.outputs
}

module "main" {
  source = "../../global/service"

  project     = "hexchess"
  environment = "uat"
  aws_region  = "us-east-1"
  app_name    = "migrator"

  desired_task_count = 1
  task_cpu           = 256
  task_memory        = 512

  vpc_id              = local.infra.vpc_id
  private_subnets_ids = local.infra.private_subnets_ids
  cluster_arn         = local.infra.cluster_arn
  alb_listener_arn    = local.infra.alb_listener_arn
  security_group_id   = local.infra.worker_security_group_id
  execution_role_arn  = local.infra.ecs_task_execution_role_arn
  task_role_arn       = local.infra.ecs_task_migrator_role_arn

  container_image = "938864279852.dkr.ecr.us-east-1.amazonaws.com/releases/hexchess/migrator"
  image_tag       = var.commit_sha

  env_vars = {
    DB_URL: "postgres://db_migrator@hexchess-sor.cluster-c10fqqu5dr0g.us-east-1.rds.amazonaws.com/hexchess?sslmode=require",

    ACTIVE_PROFILE: "test"
  }
}