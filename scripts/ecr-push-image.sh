AWS_ACCOUNT="${AWS_ACCOUNT:-938864279852}"
DOCKERFILE="${DOCKERFILE:-Migrator}"
CONTAINER="${CONTAINER:-releases/hexchess/migrator}"

COMMIT=$(git rev-parse HEAD)

aws ecr get-login-password --region us-east-1 | \
  docker login --username AWS --password-stdin "${AWS_ACCOUNT}.dkr.ecr.us-east-1.amazonaws.com"

docker build --progress=plain \
-f "./docker/${DOCKERFILE}.Dockerfile" \
-t "${AWS_ACCOUNT}.dkr.ecr.us-east-1.amazonaws.com/${CONTAINER}:${COMMIT}" .

docker push "${AWS_ACCOUNT}.dkr.ecr.us-east-1.amazonaws.com/${CONTAINER}:${COMMIT}"

echo "Uploaded with commit ${COMMIT}"