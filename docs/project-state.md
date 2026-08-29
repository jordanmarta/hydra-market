# Hydra Market — Project State

Este documento registra o estado atual do projeto na `main`. Ele deve ser atualizado conforme o sistema evoluir.

## Arquitetura atual

O Hydra Market é uma API HTTP monolítica escrita em Go, usando `net/http`, PostgreSQL e `database/sql` com o driver `pgx`.

O código é organizado por domínio:

- `cmd/api`: composição das dependências, rotas e inicialização da aplicação;
- `internal/product`: modelo, handler HTTP e repository de produtos;
- `internal/inventory`: modelo, handler HTTP e repository de estoque;
- `internal/order`: modelo, handler HTTP, service de compra e repository de pedidos;
- `db/migrations`: definição incremental do schema PostgreSQL;
- `scripts`: experimentos e reproduções executáveis;
- `docs/problems`: registro de problemas técnicos investigados;
- `docs/adr`: espaço reservado para decisões arquiteturais.

A aplicação usa repositories para acesso ao banco. O fluxo de pedidos possui uma camada de service para coordenar produto, estoque e persistência do pedido. A composição é manual em `cmd/api/main.go`. A conexão com o banco e a porta HTTP estão fixas no código.

## Funcionalidades existentes

- health check da API;
- criação de produto;
- definição ou substituição da quantidade em estoque de um produto;
- criação de pedido com um único produto;
- captura de preço e moeda do produto no item do pedido;
- rejeição de compra sem estoque suficiente;
- decremento atômico de estoque sob concorrência.

## Endpoints

| Método | Rota | Comportamento |
|---|---|---|
| `GET` | `/health` | Retorna o estado básico da API. |
| `POST` | `/products` | Cria um produto. |
| `PUT` | `/inventory/{id}` | Define a quantidade em estoque do produto. |
| `POST` | `/orders` | Cria um pedido para um produto e uma quantidade. |

## Fluxo atual de compra

1. `POST /orders` recebe `product_id` e `quantity`.
2. O service rejeita quantidades menores ou iguais a zero.
3. O produto é consultado para obter identificador, preço e moeda.
4. O estoque é decrementado por um `UPDATE` condicional atômico.
5. Se não houver estoque suficiente, a API retorna `409 Conflict`.
6. Um pedido com status `CREATED` é inserido em `orders`.
7. Um item com produto, quantidade, preço unitário e moeda é inserido em `order_items`.
8. O pedido criado é retornado com `201 Created`.

O modelo de pedido suporta uma lista de itens, mas o endpoint atual cria somente um item por pedido.

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

## Dívidas e problemas conhecidos

- O decremento de estoque e a criação do pedido não pertencem à mesma transação.
- A criação de `orders` e de seus `order_items` também não é transacional.
- Uma falha após o decremento pode consumir estoque sem criar o pedido completo.
- A configuração do banco e a porta da API estão fixas em `cmd/api/main.go`.
- As migrations precisam ser aplicadas manualmente; o `docker-compose.yml` apenas inicializa o PostgreSQL.
- Não há testes automatizados no repositório; o script existente exercita concorrência contra a aplicação e o banco em execução.
- O `README.md` da raiz ainda está vazio.

## Próximo problema candidato

Investigar a falta de atomicidade entre o decremento do estoque e a criação do pedido.

Antes de escolher uma solução, o problema deve ser reproduzido de forma controlada, com evidências do estado inconsistente. Depois devem ser comparadas as alternativas e seus trade-offs, sem assumir antecipadamente uma decisão arquitetural.
