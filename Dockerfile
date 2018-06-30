FROM golang

COPY . /go/src/app
WORKDIR /go/src/app

RUN go get ./
RUN go build

CMD app

EXPOSE 5353