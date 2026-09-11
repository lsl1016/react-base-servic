FROM node:20.13.1-slim AS sdk-builder

WORKDIR /apps/web/sdk/

COPY web/sdk/package.json  ./
RUN npm i
COPY web/sdk/ ./
RUN npm run build

FROM golang:1.23-alpine AS builder

ARG APP_NAME
ENV APP_NAME=$APP_NAME

WORKDIR $GOPATH/${APP_NAME}/

COPY go.mod $GOPATH/${APP_NAME}/
COPY go.sum $GOPATH/${APP_NAME}/
RUN go mod download
COPY . $GOPATH/${APP_NAME}/
COPY --from=sdk-builder /apps/web/sdk/dist $GOPATH/${APP_NAME}/web/sdk/dist
RUN go build -o /usr/local/bin/react-base-service main.go

FROM alpine:3.20

ARG APP_NAME
ENV APP_NAME=$APP_NAME

WORKDIR /usr/local/bin/

COPY --from=builder /usr/local/bin/react-base-service /usr/local/bin/

CMD ["/usr/local/bin/react-base-service"]
