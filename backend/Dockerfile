FROM golang:1.23-alpine AS build
WORKDIR /src
COPY . .
RUN go mod tidy && CGO_ENABLED=0 go build -o /crm .
FROM alpine:3.20
COPY --from=build /crm /crm
EXPOSE 8080
CMD ["/crm"]
