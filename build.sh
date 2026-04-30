git pull

# Load .env if present (supplies optional GOPROXY, GOSUMDB, and proxy build args)
if [ -f .env ]; then
  set -a
  source .env
  set +a
fi

docker compose -f docker-compose-prod.yml build
docker compose -f docker-compose-prod.yml up -d
go build