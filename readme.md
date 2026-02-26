compile:

`
OOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X main.BuildTime=$(date +%Y-%m-%dT%H:%M:%S)" -o moviebot ./cmd
`

generate image:

`
sudo docker build -t moviebot .
`

in compose.yml
```
services:
  ############################ MOVIEBOT  #######################################
  moviebot:
    image: moviebot:latest
    container_name: moviebot
    volumes:
      - /home/goob/containers/config/moviebot:/config
    environment:
      - TZ=Etc/UTC
    restart: unless-stopped
    ```

