# 002 — Falta de atomicidade entre estoque e pedido

## Contexto

O fluxo de compra precisa preservar uma única regra de negócio: ou o estoque é
decrementado e o pedido completo é persistido, ou nenhuma dessas alterações deve
permanecer no banco.

Atualmente, `inventory`, `orders` e `order_items` estão no mesmo PostgreSQL, mas
as operações que os alteram não compartilham uma transação explícita.

## Fluxo original

O service de criação de pedido executa:

```text
buscar produto
→ decrementar estoque
→ criar order
→ criar order_items
```

`Inventory.Repository.DecreaseStock` executa o decremento por meio de `*sql.DB`.
Depois, `Order.Repository.Create` também usa `*sql.DB` para inserir o pedido e
seus itens. Cada instrução concluída com sucesso é confirmada independentemente
pelo PostgreSQL.

## Experimento

Script utilizado:

```bash
./scripts/002-order-atomicity-failure.sh
```

O experimento:

1. verifica se a API e o produto informado estão disponíveis;
2. define o estoque do produto como 10;
3. registra estoque e quantidade de itens de pedido antes da compra;
4. instala temporariamente uma constraint que rejeita novos pedidos `CREATED`;
5. a constraint provoca uma falha controlada no PostgreSQL;
6. executa `POST /orders` para comprar uma unidade;
7. registra o status HTTP, o estoque e a quantidade de itens de pedido;
8. remove obrigatoriamente a constraint por meio de `trap`.

A falha é instalada apenas no banco local de desenvolvimento e afeta qualquer
tentativa concorrente de criar pedido enquanto o experimento estiver em execução.
Ela não exige alteração no código de produção e é removida mesmo se o script for
interrompido ou falhar.

## Evidências

Execução manual anterior à correção, utilizando o produto `1` e estoque inicial
igual a 10:

```text
Before:
  Stock:       10
  Order items: 165

After:
  HTTP status: 500
  Stock:       9
  Order items: 165

INCONSISTENCY CONFIRMED
The order failed, no order item was created, and inventory decreased.

Removing controlled failure...
Controlled failure removed.
```

O número de itens de pedido permaneceu em 165, enquanto o estoque caiu de 10
para 9. O endpoint respondeu com `500 Internal Server Error`.

O final do output também confirmou que o `trap` removeu a constraint temporária
depois do experimento.

## Comportamento observado

O sistema confirmou o decremento do estoque antes de tentar persistir o pedido.
Quando a constraint rejeitou o `INSERT INTO orders`, a API retornou erro e nenhum
novo item de pedido foi criado, mas o decremento anterior não foi desfeito.

O banco passou a representar uma unidade consumida sem possuir o pedido
correspondente.

## Hipótese

Se a persistência do pedido falhar depois de `DecreaseStock`, o endpoint retorna
erro, mas o decremento permanece confirmado no banco. Nesse caso, o sistema
indica uma unidade consumida sem possuir o pedido correspondente.

O experimento confirmou essa hipótese.

## Causa raiz

A causa raiz é a ausência de uma transaction boundary que envolva todo o caso de
uso. O decremento do estoque, a criação de `orders` e a criação de `order_items`
são executados como operações independentes por meio de `*sql.DB`.

O `UPDATE` condicional de estoque é atômico no nível da própria instrução e evita
o lost update estudado no Problema 001. Porém, depois que essa instrução termina,
sua alteração já está confirmada no PostgreSQL.

A falha posterior no `INSERT INTO orders` não participa de uma transação externa
capaz de desfazer o decremento. Por isso, operações individualmente corretas não
formam uma unidade atômica para o caso de uso completo.


## Alternativas consideradas

### 1. Compensação manual

Depois de uma falha na criação do pedido, a aplicação poderia executar uma operação inversa para devolver a quantidade ao estoque.

Essa abordagem pode ser necessária quando as operações pertencem a bancos ou serviços diferentes, mas não oferece atomicidade real. A compensação também pode falhar, exige idempotência e cria uma janela em que o estado permanece inconsistente.

Como `inventory`, `orders` e `order_items` estão no mesmo PostgreSQL, esse custo não se justifica para o estado atual do Hydra.

### 2. Transação local PostgreSQL

O PostgreSQL pode executar todas as escritas dentro da mesma transação:

```text
BEGIN
→ decrementar estoque
→ criar order
→ criar order_items
COMMIT
```

Se qualquer operação falhar, `ROLLBACK` desfaz todas as alterações realizadas desde o `BEGIN`. Essa alternativa fornece a propriedade de atomicidade necessária usando o banco que já coordena todo o estado envolvido.

### 3. Cada repository controla sua própria transação

Uma transação no `InventoryRepository` e outra no `OrderRepository` não resolveriam o problema:

```text
transação de inventory → COMMIT
transação de order     → ROLLBACK
```

O estoque continuaria reduzido. Uma transação interna no `OrderRepository` poderia tornar `orders` e `order_items` atômicos entre si, mas ainda não incluiria o estoque.

Além disso, cada repository conhece apenas sua parte da persistência e não sabe quando o caso de uso completo terminou.

### 4. Transaction boundary coordenada pelo service

O `order.Service.Create` conhece todas as operações que formam a compra. Ele pode iniciar uma transação e fornecer a mesma `*sql.Tx` aos repositories de inventory e order.

Os repositories continuam responsáveis pelos seus SQLs. O service passa a ser responsável apenas por decidir quando a unidade de negócio deve confirmar ou desfazer suas alterações.

## Decisão

Foi escolhida uma transação local PostgreSQL coordenada pelo `order.Service.Create`.

Essa decisão combina as alternativas 2 e 4:

- o PostgreSQL fornece `BEGIN`, `COMMIT` e `ROLLBACK`;
- o service define a transaction boundary do caso de uso;
- `InventoryRepository.DecreaseStock` participa por meio de `*sql.Tx`;
- `OrderRepository.Create` usa a mesma `*sql.Tx` para inserir `orders` e todos os `order_items`.

Não foi criado um Transaction Manager ou Unit of Work genérico. O service recebe `*sql.DB` diretamente e explicita o ciclo de vida da transação. Esse acoplamento foi aceito por ser simples, visível e compatível com o tamanho atual do projeto.

## Implementação conceitual

```text
buscar produto
→ BeginTx
→ defer Rollback
→ DecreaseStock(tx)
→ CreateOrder(tx)
→ CreateOrderItems(tx)
→ Commit
```

O rollback deferido protege todos os retornos antecipados, incluindo estoque insuficiente e erros de persistência. O commit acontece explicitamente apenas depois que todas as operações terminam e seu erro também é propagado.

O produto continua sendo consultado antes do `BeginTx`. Isso mantém a transação curta e é suficiente para o comportamento atual, no qual não existe fluxo de alteração concorrente de preço ou moeda.

## Trade-offs

### Benefícios

- atomicidade real entre estoque, order e items;
- ausência de operações de compensação;
- transaction boundary visível no caso de uso;
- SQL permanece nos repositories de cada domínio;
- solução usa apenas recursos já disponíveis no PostgreSQL e em `database/sql`.

### Custos e limitações

- o service passa a depender diretamente de `database/sql`;
- métodos transacionais dos repositories recebem `*sql.Tx`;
- a transação mantém uma conexão do pool ocupada até commit ou rollback;
- operações lentas dentro da transação aumentariam contenção;
- a solução depende de todos os dados envolvidos estarem no mesmo PostgreSQL.

Se inventory e order forem separados em bancos ou serviços diferentes, uma transação local não poderá coordená-los. Nesse cenário futuro, compensação, reservas, idempotência ou padrões distribuídos deverão ser avaliados a partir do problema concreto. Eles não são necessários para a arquitetura atual.

## Validação após a correção

O mesmo script e a mesma falha controlada usados na reprodução foram executados novamente:

```text
Before:
  Stock:       10
  Order items: 165

After:
  HTTP status: 500
  Stock:       10
  Order items: 165
```

O pedido continuou falhando com HTTP 500 e nenhum item foi criado, mas o estoque permaneceu em 10. O rollback preservou o estado anterior.

O fluxo normal também foi validado manualmente sem a constraint temporária. A API retornou sucesso, o pedido e seu item foram persistidos e o estoque foi decrementado, confirmando o caminho de commit.

O script passou a reconhecer os dois resultados conhecidos:

- `INCONSISTENCY CONFIRMED`: pedido falha e estoque diminui;
- `ATOMICITY PRESERVED`: pedido falha e estoque permanece igual.

## Lições aprendidas

- atomicidade de uma instrução SQL não implica atomicidade do caso de uso;
- abrir uma transação não basta: todas as queries precisam usar a mesma `*sql.Tx`;
- a transaction boundary deve acompanhar a unidade de negócio;
- repositories diferentes podem participar da mesma transação sem misturar seus SQLs;
- rollback é o comportamento seguro padrão e commit deve ser uma decisão explícita;
- erros de `BeginTx` e `Commit` fazem parte do resultado do caso de uso;
- transação local e compensação resolvem contextos diferentes;
- abstrações como Unit of Work devem surgir de uma dor observada, não apenas da possibilidade de uso futuro.
