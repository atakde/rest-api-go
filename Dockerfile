FROM golang:latest

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
COPY .env .

RUN go build -o /rest-api-go

EXPOSE 8080

CMD [ "/rest-api-go" ]