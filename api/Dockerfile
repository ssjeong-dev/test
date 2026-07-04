# --- build stage ---
FROM golang:1.25-alpine AS build

RUN apk add --no-cache git ca-certificates
ENV GOPROXY=https://proxy.golang.org,direct
ENV GOTOOLCHAIN=auto

WORKDIR /app

COPY go.mod ./
COPY *.go ./
# go.sum 이 없어도 빌드되도록 의존성을 해석하고 go.sum 을 생성한다.
RUN go mod tidy && go build -o /usr/local/bin/api

# --- runtime stage ---
FROM alpine:3.20

COPY --from=build /usr/local/bin/api /usr/local/bin/api

EXPOSE 8081
CMD ["/usr/local/bin/api"]
