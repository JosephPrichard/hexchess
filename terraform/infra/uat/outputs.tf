output "vpc_id" {
  value = module.infra.vpc_id
}

output "private_subnets_ids" {
  value = module.infra.private_subnets_ids
}

output "cluster_arn" {
  value = module.infra.cluster_arn
}

output "alb_listener_arn" {
  value = module.infra.alb_listener_arn
}

output "listener_security_group_id" {
  value = module.infra.listener_security_group_id
}

output "worker_security_group_id" {
  value = module.infra.worker_security_group_id
}

output "ecs_task_execution_role_arn" {
  value = module.infra.ecs_task_execution_role_arn
}

output "ecs_task_migrator_role_arn" {
  value = module.infra.ecs_task_migrator_role_arn
}

output "ecs_task_app_role_arn" {
  value = module.infra.ecs_task_app_role_arn
}

output "ecs_task_frontend_role_arn" {
  value = module.infra.ecs_task_frontend_role_arn
}