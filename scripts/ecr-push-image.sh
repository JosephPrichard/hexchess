# Note(Joseph): Must be run from inside root, not this directory (Docker images reference things relative to root)

AWS_ACCOUNT="${AWS_ACCOUNT:-938864279852}"
DOCKERFILE="${DOCKERFILE:-Frontend}"
SVC="frontend"
CONTAINER="releases/hexchess/${SVC}"

COMMIT=$(git rev-parse HEAD)

aws ecr get-login-password --region us-east-1 | \
  docker login --username AWS --password-stdin "${AWS_ACCOUNT}.dkr.ecr.us-east-1.amazonaws.com"

docker build --progress=plain \
--build-arg SERVICE="${SVC}" \
-f "./docker/app/${DOCKERFILE}.Dockerfile" \
-t "${AWS_ACCOUNT}.dkr.ecr.us-east-1.amazonaws.com/${CONTAINER}:${COMMIT}" .

docker push "${AWS_ACCOUNT}.dkr.ecr.us-east-1.amazonaws.com/${CONTAINER}:${COMMIT}"

echo "Uploaded with commit ${COMMIT}"