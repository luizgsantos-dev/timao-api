# Build
FROM golang:1.27.3-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /timao-api ./cmd/api
# Pasta do banco, que a imagem final (sem shell) não consegue criar.
RUN mkdir -p /dados

# Runtime: imagem mínima, sem shell. O fuso vem embutido via time/tzdata.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /timao-api /timao-api
COPY --from=build --chown=nonroot:nonroot /dados /dados
# O banco fica num volume para sobreviver a reinícios. Sem banco, a API importa
# sozinha ao subir (uns 10 minutos) e depois atualiza toda semana.
ENV DB_FILE=/dados/timao.db
VOLUME /dados
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/timao-api"]
