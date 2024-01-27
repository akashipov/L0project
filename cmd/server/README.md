docker run -p 4222:4222 --name nats-server -ti nats:latest
- to run nats server inside the docker (fastest solution for me)

docker-compose up -d
- to create postgres db instance for our program

go run cmd/publisher/main.go
- to run publisher of nats server

go run cmd/server/main.go -p <pwd for postgres db> -n <host>:<port>
- to run subscriber
