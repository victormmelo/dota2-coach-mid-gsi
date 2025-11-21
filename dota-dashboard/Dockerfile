FROM node:22-alpine

WORKDIR /app

# Copiar dependências
COPY package.json package-lock.json* ./
RUN npm install

# Copiar código
COPY . .

# Expor porta
EXPOSE 3000

# Rodar em modo dev (para você ver alterações se configurar volumes, 
# mas em produção seria npm run build && npm start)
CMD ["npm", "run", "dev"]