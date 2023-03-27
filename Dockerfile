FROM golang:1.17-alpine

WORKDIR /app

RUN apk add --no-cache git
RUN apk add build-base
# copy directory files i.e all files ending with .go
COPY . ./

RUN go mod tidy

# compile application
RUN go build -o bin/thresher main.go

# tells Docker that~~ the container listens on specified network ports at runtime
EXPOSE 59392

CMD tail -f /dev/null