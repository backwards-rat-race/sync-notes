FROM node as ui
WORKDIR /sync-notes-ui
COPY sync-notes-ui/ ./
RUN yarn
RUN yarn build

FROM golang as api
WORKDIR /sync-notes-api/
COPY sync-notes-api/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o app .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=api /sync-notes-api/app ./
COPY --from=ui /sync-notes-ui/dist ./static
EXPOSE 3000
ENTRYPOINT ./app
