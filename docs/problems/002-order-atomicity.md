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
  Stock:  10
  Orders: 165

After:
  HTTP status: 500
  Stock:       9
  Orders:      165

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
