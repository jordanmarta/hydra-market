# Hydra Market — Project State

Este documento registra o estado atual do projeto na `main`. Ele deve ser atualizado conforme o sistema evoluir.

## Arquitetura atual

O Hydra Market é uma API HTTP monolítica escrita em Go, usando `net/http`, PostgreSQL e `database/sql` com o driver `pgx`.

O código é organizado por domínio:

- `cmd/api`: composição das dependências, rotas e inicialização da aplicação;
- `internal/product`: modelo, handler HTTP e repository de produtos;
- `internal/inventory`: modelo, handler HTTP e repository de estoque;
- `internal/order`: modelo, handler HTTP, service de compra e repository de pedidos;
- `internal/user`: modelo, handler HTTP e repository de usuários;
- `db/migrations`: definição incremental do schema PostgreSQL;
- `scripts`: experimentos e reproduções executáveis;
- `docs/problems`: registro de problemas técnicos investigados;
- `docs/adr`: espaço reservado para decisões arquiteturais.

A aplicação usa repositories para acesso ao banco. O fluxo de pedidos possui uma camada de service para coordenar produto, estoque e persistência do pedido. Esse service também controla a transação local PostgreSQL do caso de uso, compartilhada pelos repositories por meio de `*sql.Tx`. A composição é manual em `cmd/api/main.go`. A conexão com o banco e a porta HTTP estão fixas no código.

## Funcionalidades existentes

- health check da API;
- criação de produto;
- definição ou substituição da quantidade em estoque de um produto;
- criação de pedido com múltiplos produtos;
- captura de preço e moeda de cada produto no item do pedido;
- rejeição de compra sem estoque suficiente;
- decremento atômico de estoque sob concorrência;
- criação transacional de estoque, pedido e itens.
- cadastro e consulta de usuários;
- associação de pedidos a usuários;

## Endpoints

| Método | Rota | Comportamento |
|---|---|---|
| `GET` | `/health` | Retorna o estado básico da API. |
| `POST` | `/products` | Cria um produto. |
| `PUT` | `/inventory/{id}` | Define a quantidade em estoque do produto. |
| `POST` | `/orders` | Cria um pedido associado a um usuário contendo um ou mais produtos, cada um com `product_id` e `quantity`. |
| `POST` | `/users` | Cria um usuário. |
| `GET` | `/users/{id}` | Consulta um usuário pelo identificador. |

## Fluxo atual de compra

1. `POST /orders` recebe `user_id` e `items[]`.
2. Cada item do payload possui `product_id` e `quantity`.
3. O service rejeita quantidades menores ou iguais a zero.
4. Todos os produtos da lista são consultados antes da transação para obter identificador, preço e moeda.
5. O usuário é consultado e precisa existir.
6. O service inicia uma transação PostgreSQL.
7. O estoque de cada item é decrementado usando a mesma transação.
8. Se qualquer item não tiver estoque suficiente, a API retorna `409 Conflict` e toda a transação é desfeita.
9. Uma única order associada ao usuário, com status `CREATED`, é inserida em `orders`.
10. Todos os itens são persistidos em `order_items` com produto, quantidade, preço unitário e moeda.
11. O commit ocorre apenas após a criação completa do pedido e o service retorna o pedido com `201 Created`.

Qualquer falha depois do início da transação provoca rollback e preserva o estado anterior.

O campo `orders.user_id` foi adicionado como nullable para preservar pedidos históricos existentes, mas o fluxo atual da aplicação exige um usuário válido para novos pedidos.

## Problema 001 — concluído

O primeiro fluxo de compra sofria de lost update: requisições concorrentes podiam ler o mesmo estoque, validar o mesmo estado e sobrescrever decrementos umas das outras, permitindo overselling.

A solução implementada foi substituir o ciclo separado de leitura, validação e escrita por um único `UPDATE` condicional no PostgreSQL:

```sql
UPDATE inventory
SET quantity = quantity - $2,
    updated_at = NOW()
WHERE product_id = $1
  AND quantity >= $2
RETURNING quantity;
```

A validação e o decremento agora são uma operação atômica coordenada pelo banco, inclusive quando existem múltiplas instâncias da aplicação. A investigação completa está em `docs/problems/001-concurrent-inventory-lost-update.md`.

## Problema 002 — concluído

O fluxo de compra decrementava o estoque antes de persistir o pedido, sem uma transação compartilhada. Uma falha controlada no `INSERT INTO orders` produziu HTTP 500, manteve a quantidade de itens de pedido e reduziu o estoque de 10 para 9.

A solução foi criar uma transação local PostgreSQL na boundary do `order.Service.Create`. O service inicia a transação, os repositories de inventory e order executam todas as escritas por meio da mesma `*sql.Tx`, e o commit ocorre somente depois da criação completa do pedido. Qualquer erro provoca rollback.

Após a correção, o mesmo experimento produziu HTTP 500, manteve a quantidade de itens de pedido e preservou o estoque em 10. O fluxo normal de compra também foi validado. A investigação completa está em `docs/problems/002-order-atomicity.md`.

## Dívidas e problemas conhecidos

- A configuração do banco e a porta da API estão fixas em `cmd/api/main.go`.
- As migrations precisam ser aplicadas manualmente; o `docker-compose.yml` apenas inicializa o PostgreSQL.
- Não há testes automatizados no repositório; os scripts existentes exercitam cenários contra a aplicação e o banco em execução.
- O `README.md` da raiz ainda está vazio.
