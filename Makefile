up:
	docker compose -f docker-compose.yml pull
	docker compose -f docker-compose.yml up -d --build

down:
	docker compose -f docker-compose.yml down

up-debug:
	docker compose -f docker-compose.dev.yml up -d --build

down-debug:
	docker compose -f docker-compose.dev.yml down

add-migrate: instal-modules
	migrate create -ext sql -dir migrations $(name)

fmt: instal-modules
	gofumpt -w ./.
	swag fmt
	golangci-lint config verify
	golangci-lint run --fix

instal-modules:
	go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	go install go.uber.org/mock/mockgen@latest
	go install mvdan.cc/gofumpt@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest