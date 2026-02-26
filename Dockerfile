FROM alpine:3.20

# Ensure UTF-8 locales for emojis
ENV LANG=C.UTF-8
ENV LC_ALL=C.UTF-8

WORKDIR /app

COPY moviebot /app/moviebot

CMD ["/app/moviebot"]