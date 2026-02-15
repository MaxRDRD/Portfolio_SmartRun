include .env
export

service-run:
	go run main.go

migrate-up:
	powershell -Command "migrate -path migrations -database ${CONN_STRING} up"

migrate-down:
	powershell -Command "migrate -path migrations -database ${CONN_STRING} down"
