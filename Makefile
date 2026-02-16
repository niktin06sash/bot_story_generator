include .env
export
.PHONY: all start tests stop run logs status

all: start

migrate-up:
	migrate -path ./internal/schema -database "${DATABASE_CONNECT_URL}" up
migrate-down:
	migrate -path ./internal/schema -database "${DATABASE_CONNECT_URL}" down
tests:
	cd "internal/service/service_test/" && go test -v ./...
build:
	docker build -t bot_story_generator .
start:
	docker run --name bot_story_generator-container bot_story_generator
stop:
	docker stop bot_story_generator
run:
	build tests migrate-up start
logs:
	docker logs -f bot_story_generator-container
status:
	docker ps --filter "name=bot_story_generator-container"