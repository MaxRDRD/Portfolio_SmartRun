include .env
export

service-run:
	go run ./cmd/app

migrate-up:
	powershell -Command "migrate -path migrations -database ${CONN_STRING} up"

migrate-down:
	powershell -Command "migrate -path migrations -database ${CONN_STRING} down"
