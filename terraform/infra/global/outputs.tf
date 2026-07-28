output "vpc_id" {
  value = module.vpc.vpc_id
}

output "private_subnets_ids" {
  value = module.vpc.private_subnets
}

output "cluster_arn" {
  value = aws_ecs_cluster.main.arn
}

output "alb_listener_arn" {
  value = aws_lb_listener.private_http.arn
}

output "listener_security_group_id" {
  value = aws_security_group.ecs_service_listener.id
}

output "worker_security_group_id" {
  value = aws_security_group.ecs_service_worker.id
}

output "ecs_task_execution_role_arn" {
  value = aws_iam_role.ecs_task_execution_role.arn
}

output "ecs_task_migrator_role_arn" {
  value = aws_iam_role.ecs_task_migrator_role.arn
}

output "ecs_task_app_role_arn" {
  value = aws_iam_role.ecs_task_app_role.arn
}

output "ecs_task_frontend_role_arn" {
  value = aws_iam_role.ecs_task_frontend_role.arn
}