package orders

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gfotev/loadout/internal/platform/db"
)

type store struct{}

// cartLines returns the lines of the cart with this token hash, the last added first. A cart
// that doesn't exist has none.
func (store) cartLines(ctx context.Context, q db.DBTX, tokenHash []byte) ([]CartLine, error) {
	query := `
		SELECT l.product_id, l.slug, l.name, l.quantity
		FROM orders.cart_lines l
		JOIN orders.carts c ON c.id = l.cart_id
		WHERE c.token_hash = $1
		ORDER BY l.added_at DESC, l.product_id`

	rows, err := q.Query(ctx, query, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("orders: cart lines: %w", err)
	}
	lines, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (CartLine, error) {
		var l CartLine
		err := row.Scan(
			&l.ProductID,
			&l.Slug,
			&l.Name,
			&l.Quantity,
		)
		return l, err
	})
	if err != nil {
		return nil, fmt.Errorf("orders: cart lines: %w", err)
	}
	return lines, nil
}

// lockCart finds the cart with this token hash and locks it until the transaction ends, so two
// adds to one cart don't both pass the limits. It reports false if there's no such cart.
func (store) lockCart(ctx context.Context, q db.DBTX, tokenHash []byte) (uuid.UUID, bool, error) {
	query := `SELECT id FROM orders.carts WHERE token_hash = $1 FOR UPDATE`

	var id uuid.UUID
	err := q.QueryRow(ctx, query, tokenHash).Scan(&id)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return uuid.Nil, false, nil
	case err != nil:
		return uuid.Nil, false, fmt.Errorf("orders: lock cart: %w", err)
	}
	return id, true, nil
}

func (store) insertCart(ctx context.Context, q db.DBTX, tokenHash []byte) (uuid.UUID, error) {
	query := `INSERT INTO orders.carts (token_hash) VALUES ($1) RETURNING id`

	var id uuid.UUID
	if err := q.QueryRow(ctx, query, tokenHash).Scan(&id); err != nil {
		return uuid.Nil, fmt.Errorf("orders: insert cart: %w", err)
	}
	return id, nil
}

func (store) touchCart(ctx context.Context, q db.DBTX, cartID uuid.UUID) error {
	query := `UPDATE orders.carts SET updated_at = now() WHERE id = $1`

	if _, err := q.Exec(ctx, query, cartID); err != nil {
		return fmt.Errorf("orders: touch cart: %w", err)
	}
	return nil
}

// quantities maps the products in a cart to their quantity.
func (store) quantities(ctx context.Context, q db.DBTX, cartID uuid.UUID) (map[uuid.UUID]int, error) {
	query := `SELECT product_id, quantity FROM orders.cart_lines WHERE cart_id = $1`

	rows, err := q.Query(ctx, query, cartID)
	if err != nil {
		return nil, fmt.Errorf("orders: cart quantities: %w", err)
	}
	defer rows.Close()
	have := map[uuid.UUID]int{}
	for rows.Next() {
		var id uuid.UUID
		var qty int
		if err := rows.Scan(
			&id,
			&qty,
		); err != nil {
			return nil, fmt.Errorf("orders: cart quantities: %w", err)
		}
		have[id] = qty
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("orders: cart quantities: %w", err)
	}
	return have, nil
}

// addLine adds quantity of a product to a cart, on top of a line already there. It moves the
// line to the top and refreshes its slug and name.
func (store) addLine(ctx context.Context, q db.DBTX, cartID, productID uuid.UUID, slug, name string, quantity int) error {
	query := `
		INSERT INTO orders.cart_lines (cart_id, product_id, slug, name, quantity)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (cart_id, product_id) DO UPDATE
		SET slug = EXCLUDED.slug,
		    name = EXCLUDED.name,
		    quantity = orders.cart_lines.quantity + EXCLUDED.quantity,
		    added_at = now()`

	if _, err := q.Exec(ctx, query, cartID, productID, slug, name, quantity); err != nil {
		return fmt.Errorf("orders: add cart line: %w", err)
	}
	return nil
}

// setQuantity sets a line's quantity in the cart with this token hash. It reports false if the
// cart or the line doesn't exist.
func (store) setQuantity(ctx context.Context, q db.DBTX, tokenHash []byte, productID uuid.UUID, quantity int) (bool, error) {
	query := `
		WITH c AS (
			UPDATE orders.carts SET updated_at = now() WHERE token_hash = $1 RETURNING id
		)
		UPDATE orders.cart_lines l SET quantity = $3
		FROM c
		WHERE l.cart_id = c.id AND l.product_id = $2`

	tag, err := q.Exec(ctx, query, tokenHash, productID, quantity)
	if err != nil {
		return false, fmt.Errorf("orders: set cart quantity: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (store) removeLine(ctx context.Context, q db.DBTX, tokenHash []byte, productID uuid.UUID) error {
	query := `
		WITH c AS (
			UPDATE orders.carts SET updated_at = now() WHERE token_hash = $1 RETURNING id
		)
		DELETE FROM orders.cart_lines l
		USING c
		WHERE l.cart_id = c.id AND l.product_id = $2`

	if _, err := q.Exec(ctx, query, tokenHash, productID); err != nil {
		return fmt.Errorf("orders: remove cart line: %w", err)
	}
	return nil
}
