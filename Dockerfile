# Usar imagem leve do Go
FROM golang:1.25-alpine

WORKDIR /app

# Copiar dependências
COPY go.mod go.sum ./
RUN go mod download

# Copiar código fonte
COPY . .

# Expor porta
EXPOSE 8080

# Rodar o app
CMD ["go", "run", "main.go"]