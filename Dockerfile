FROM golang:1.18-alpine as builder
LABEL stage=builder
WORKDIR /app

COPY go.mod ./
COPY go.sum ./

RUN go mod download

COPY . .

RUN go build -o /cloudocs .

FROM alpine

COPY --from=builder /cloudocs /cloudocs

EXPOSE 8080

CMD [ "/cloudocs" ]
