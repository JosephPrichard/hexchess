#!/usr/bin/env fish
# Note(Joseph): Must be run from inside root, not this directory (Docker images reference things relative to root)

if not set -q AWS_ACCOUNT
    set -g AWS_ACCOUNT 938864279852
end
if not set -q DOCKERFILE
    set -g DOCKERFILE "BackendService"
end
if not set -q SVC
    set -g SVC "api"
end

set CONTAINER "releases/hexchess/$SVC"

if not set -q COMMIT
    set -g COMMIT (git rev-parse HEAD)
end

aws ecr get-login-password --region us-east-1 | \
  docker login --username AWS --password-stdin "$AWS_ACCOUNT.dkr.ecr.us-east-1.amazonaws.com"

docker build --progress=plain \
--build-arg SERVICE="$SVC" \
-f "./docker/app/$DOCKERFILE.Dockerfile" \
-t "$AWS_ACCOUNT.dkr.ecr.us-east-1.amazonaws.com/$CONTAINER:$COMMIT" .

set CONTAINER_REGISTRY "$AWS_ACCOUNT.dkr.ecr.us-east-1.amazonaws.com/$CONTAINER"
docker push "$CONTAINER_REGISTRY:$COMMIT"
echo "Uploaded to container registry $CONTAINER_REGISTRY with commit $COMMIT"