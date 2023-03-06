FROM golang:1.17-alpine

WORKDIR /app

RUN apk add --no-cache git

# copy directory files i.e all files ending with .go
COPY . ./

RUN go mod tidy


# RUN go get "github.com/johnthethird/thresher/tree/master/commands"
# RUN go get github.com/johnthethird/thresher/tree/master/ulimit
# RUN go get github.com/johnthethird/thresher/tree/master/version

# compile application
RUN go build -o bin/thresher main.go
# RUN bin/thresher init avalanche fuji DAO-Treasury alice X-fuji1knjauvyjxf56tavysqnf9zxds084588nqja7j4 &&\
# 	bin/thresher init avalanche fuji DAO-Treasury bob X-fuji1uehmke49qtysde4p2ehvnpvp7sc6j8xdntrma0 &&\
# 	bin/thresher init avalanche fuji DAO-Treasury cam X-fuji13avtfecrzkhxrd8mxqcd0ehctsvqh99y6xjnr2

# tells Docker that~~ the container listens on specified network ports at runtime
EXPOSE 59392

CMD tail -f /dev/null