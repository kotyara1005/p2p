FROM golang

COPY . /go/src/app
WORKDIR /go/src/app

RUN go install

CMD app

EXPOSE 5353