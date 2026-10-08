package orders

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gfotev/loadout/internal/money"
	"github.com/gfotev/loadout/internal/platform/db"
)

type store struct{}

// cartKey finds a cart: by userID, or else by tokenHash. Exactly one is set, so the other is a
// NULL argument and `user_id = $1 OR token_hash = $2` matches on the one that is.
type cartKey struct {
	userID    *uuid.UUID
	tokenHash []byte
}

// cartLines returns the lines of the cart with this key, the last added first. A cart that
// doesn't exist has none.
func (store) cartLines(ctx context.Context, q db.DBTX, k cartKey) ([]CartLine, error) {
	query := `
		SELECT l.product_id, l.slug, l.name, l.quantity
		FROM orders.cart_lines l
		JOIN orders.carts c ON c.id = l.cart_id
		WHERE c.user_id = $1 OR c.token_hash = $2
		ORDER BY l.added_at DESC, l.product_id`

	rows, err := q.Query(ctx, query, k.userID, k.tokenHash)
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

// lockCart finds the cart with this key and locks it until the transaction ends, so two adds to
// one cart don't both pass the limits. It reports false if there's no such cart.
func (store) lockCart(ctx context.Context, q db.DBTX, k cartKey) (uuid.UUID, bool, error) {
	query := `SELECT id FROM orders.carts WHERE user_id = $1 OR token_hash = $2 FOR UPDATE`

	var id uuid.UUID
	err := q.QueryRow(ctx, query, k.userID, k.tokenHash).Scan(&id)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return uuid.Nil, false, nil
	case err != nil:
		return uuid.Nil, false, fmt.Errorf("orders: lock cart: %w", err)
	}
	return id, true, nil
}

func (store) insertCart(ctx context.Context, q db.DBTX, k cartKey) (uuid.UUID, error) {
	query := `INSERT INTO orders.carts (user_id, token_hash) VALUES ($1, $2) RETURNING id`

	var id uuid.UUID
	if err := q.QueryRow(ctx, query, k.userID, k.tokenHash).Scan(&id); err != nil {
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

// setQuantity sets a line's quantity in the cart with this key. It reports false if the cart or
// the line doesn't exist.
func (store) setQuantity(ctx context.Context, q db.DBTX, k cartKey, productID uuid.UUID, quantity int) (bool, error) {
	query := `
		WITH c AS (
			UPDATE orders.carts SET updated_at = now() WHERE user_id = $1 OR token_hash = $2 RETURNING id
		)
		UPDATE orders.cart_lines l SET quantity = $4
		FROM c
		WHERE l.cart_id = c.id AND l.product_id = $3`

	tag, err := q.Exec(ctx, query, k.userID, k.tokenHash, productID, quantity)
	if err != nil {
		return false, fmt.Errorf("orders: set cart quantity: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (store) removeLine(ctx context.Context, q db.DBTX, k cartKey, productID uuid.UUID) error {
	query := `
		WITH c AS (
			UPDATE orders.carts SET updated_at = now() WHERE user_id = $1 OR token_hash = $2 RETURNING id
		)
		DELETE FROM orders.cart_lines l
		USING c
		WHERE l.cart_id = c.id AND l.product_id = $3`

	if _, err := q.Exec(ctx, query, k.userID, k.tokenHash, productID); err != nil {
		return fmt.Errorf("orders: remove cart line: %w", err)
	}
	return nil
}

// lockCartsForMerge locks the guest cart with this token hash and the user's cart, in id order
// so two merges can't deadlock. Either id is uuid.Nil if there's no such cart.
func (store) lockCartsForMerge(ctx context.Context, q db.DBTX, tokenHash []byte, userID uuid.UUID) (guestID, userCartID uuid.UUID, err error) {
	query := `
		SELECT id, user_id IS NOT NULL
		FROM orders.carts
		WHERE token_hash = $1 OR user_id = $2
		ORDER BY id
		FOR UPDATE`

	rows, err := q.Query(ctx, query, tokenHash, userID)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("orders: lock carts for merge: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var isUser bool
		if err := rows.Scan(
			&id,
			&isUser,
		); err != nil {
			return uuid.Nil, uuid.Nil, fmt.Errorf("orders: lock carts for merge: %w", err)
		}
		if isUser {
			userCartID = id
		} else {
			guestID = id
		}
	}
	if err := rows.Err(); err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("orders: lock carts for merge: %w", err)
	}
	return guestID, userCartID, nil
}

// giveCart turns a guest cart into the user's: it's found by user from now on, not by token.
func (store) giveCart(ctx context.Context, q db.DBTX, cartID, userID uuid.UUID) error {
	query := `UPDATE orders.carts SET user_id = $2, token_hash = NULL, updated_at = now() WHERE id = $1`

	if _, err := q.Exec(ctx, query, cartID, userID); err != nil {
		return fmt.Errorf("orders: give cart to user: %w", err)
	}
	return nil
}

// mergeCart moves the lines of cart from into cart into and deletes from. A product in both gets
// the sum of the quantities, up to maxQty; past maxLines, the least recently added lines go.
func (store) mergeCart(ctx context.Context, q db.DBTX, from, into uuid.UUID, maxQty, maxLines int) error {
	query := `
		INSERT INTO orders.cart_lines (cart_id, product_id, slug, name, quantity, added_at)
		SELECT $2, product_id, slug, name, quantity, added_at
		FROM orders.cart_lines
		WHERE cart_id = $1
		ON CONFLICT (cart_id, product_id) DO UPDATE
		SET slug = EXCLUDED.slug,
		    name = EXCLUDED.name,
		    quantity = LEAST(orders.cart_lines.quantity + EXCLUDED.quantity, $3),
		    added_at = GREATEST(orders.cart_lines.added_at, EXCLUDED.added_at)`
	if _, err := q.Exec(ctx, query, from, into, maxQty); err != nil {
		return fmt.Errorf("orders: merge cart lines: %w", err)
	}

	query = `
		DELETE FROM orders.cart_lines
		WHERE cart_id = $1 AND product_id NOT IN (
			SELECT product_id FROM orders.cart_lines
			WHERE cart_id = $1
			ORDER BY added_at DESC, product_id
			LIMIT $2
		)`
	if _, err := q.Exec(ctx, query, into, maxLines); err != nil {
		return fmt.Errorf("orders: trim merged cart: %w", err)
	}

	if _, err := q.Exec(ctx, `DELETE FROM orders.carts WHERE id = $1`, from); err != nil {
		return fmt.Errorf("orders: delete merged cart: %w", err)
	}
	if _, err := q.Exec(ctx, `UPDATE orders.carts SET updated_at = now() WHERE id = $1`, into); err != nil {
		return fmt.Errorf("orders: touch merged cart: %w", err)
	}
	return nil
}

// lockOrder returns an order and locks its row until the transaction ends, or ErrNotFound.
func (store) lockOrder(ctx context.Context, q db.DBTX, id uuid.UUID) (Order, error) {
	query := `SELECT id, status, payment, paid_at, seq FROM orders.orders WHERE id = $1 FOR UPDATE`

	var o Order
	err := q.QueryRow(ctx, query, id).Scan(
		&o.ID,
		&o.Status,
		&o.Payment,
		&o.PaidAt,
		&o.Seq,
	)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Order{}, ErrNotFound
	case err != nil:
		return Order{}, fmt.Errorf("orders: lock order %s: %w", id, err)
	}
	return o, nil
}

// saveTransition stores the order after a transition and the transition's audit row.
func (store) saveTransition(ctx context.Context, q db.DBTX, from, to Order, ev Event, by Actor, at time.Time) error {
	update := `
		UPDATE orders.orders
		SET status = $2, paid_at = $3, seq = $4, updated_at = $5,
		    status_at = CASE WHEN status = $2 THEN status_at ELSE $5 END
		WHERE id = $1`
	if _, err := q.Exec(ctx, update,
		to.ID,
		to.Status,
		to.PaidAt,
		to.Seq,
		at,
	); err != nil {
		return fmt.Errorf("orders: save order %s: %w", to.ID, err)
	}

	insert := `
		INSERT INTO orders.order_transitions (order_id, seq, event, from_status, to_status, at, actor, note)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	if _, err := q.Exec(ctx, insert,
		to.ID,
		to.Seq,
		ev,
		from.Status,
		to.Status,
		at,
		by.Email,
		by.Note,
	); err != nil {
		return fmt.Errorf("orders: insert transition of order %s: %w", to.ID, err)
	}
	return nil
}

func (store) deleteCart(ctx context.Context, q db.DBTX, cartID uuid.UUID) error {
	if _, err := q.Exec(ctx, `DELETE FROM orders.carts WHERE id = $1`, cartID); err != nil {
		return fmt.Errorf("orders: delete cart: %w", err)
	}
	return nil
}

// insertOrder stores a new order waiting for its payment, with its lines, and returns its id
// and number.
func (store) insertOrder(ctx context.Context, q db.DBTX, o draft) (uuid.UUID, int64, error) {
	var userID *uuid.UUID
	if o.UserID != uuid.Nil {
		userID = &o.UserID
	}
	insert := `
		INSERT INTO orders.orders (
			status, payment, token, user_id, email, name, phone, city, postcode, address,
			subtotal_cents, delivery_cents, total_cents
		)
		VALUES ('awaiting_payment', $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, number`

	var id uuid.UUID
	var number int64
	err := q.QueryRow(ctx, insert,
		o.Payment,
		o.Token,
		userID,
		o.Email,
		o.Name,
		o.Phone,
		o.City,
		o.Postcode,
		o.Address,
		o.SubtotalCents,
		o.DeliveryCents,
		o.SubtotalCents+o.DeliveryCents,
	).Scan(
		&id,
		&number,
	)
	if err != nil {
		return uuid.Nil, 0, fmt.Errorf("orders: insert order: %w", err)
	}

	lineInsert := `
		INSERT INTO orders.order_lines (
			order_id, position, product_id, provider, provider_sku, slug, name, image, quantity,
			dealer_cents, retail_cents, vat_bp
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`
	for i, l := range o.Lines {
		if _, err := q.Exec(ctx, lineInsert,
			id,
			i,
			l.ID,
			l.Provider,
			l.ProviderSKU,
			l.Slug,
			l.Name,
			l.Image,
			l.Quantity,
			l.DealerCents,
			l.RetailCents,
			l.VATbp,
		); err != nil {
			return uuid.Nil, 0, fmt.Errorf("orders: insert order line: %w", err)
		}
	}
	return id, number, nil
}

// order returns an order with its lines, or ErrNotFound.
func (store) order(ctx context.Context, q db.DBTX, id uuid.UUID) (OrderDetail, error) {
	query := `
		SELECT id, number, token, status, payment, paid_at, created_at, user_id, email, name, phone,
		       city, postcode, address, subtotal_cents, delivery_cents, total_cents
		FROM orders.orders
		WHERE id = $1`

	var o OrderDetail
	err := q.QueryRow(ctx, query, id).Scan(
		&o.ID,
		&o.Number,
		&o.Token,
		&o.Status,
		&o.Payment,
		&o.PaidAt,
		&o.CreatedAt,
		&o.UserID,
		&o.Email,
		&o.Name,
		&o.Phone,
		&o.City,
		&o.Postcode,
		&o.Address,
		&o.SubtotalCents,
		&o.DeliveryCents,
		&o.TotalCents,
	)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return OrderDetail{}, ErrNotFound
	case err != nil:
		return OrderDetail{}, fmt.Errorf("orders: order %s: %w", id, err)
	}

	query = `
		SELECT product_id, slug, name, image, quantity, retail_cents
		FROM orders.order_lines
		WHERE order_id = $1
		ORDER BY position`
	rows, err := q.Query(ctx, query, id)
	if err != nil {
		return OrderDetail{}, fmt.Errorf("orders: lines of order %s: %w", id, err)
	}
	o.Lines, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (OrderLine, error) {
		var l OrderLine
		err := row.Scan(
			&l.ProductID,
			&l.Slug,
			&l.Name,
			&l.Image,
			&l.Quantity,
			&l.UnitCents,
		)
		l.TotalCents = l.UnitCents * money.Cents(l.Quantity)
		return l, err
	})
	if err != nil {
		return OrderDetail{}, fmt.Errorf("orders: lines of order %s: %w", id, err)
	}
	return o, nil
}

// customerOrders returns the orders of a user, and the ones placed with email when it isn't
// "", newest first.
func (store) customerOrders(ctx context.Context, q db.DBTX, userID uuid.UUID, email string) ([]OrderSummary, error) {
	query := `
		SELECT o.id, o.number, o.status, o.created_at, o.total_cents,
		       (SELECT sum(l.quantity) FROM orders.order_lines l WHERE l.order_id = o.id)
		FROM orders.orders o
		WHERE o.user_id = $1 OR ($2 <> '' AND o.email = $2::citext)
		ORDER BY o.created_at DESC, o.number DESC`

	rows, err := q.Query(ctx, query, userID, email)
	if err != nil {
		return nil, fmt.Errorf("orders: customer orders: %w", err)
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (OrderSummary, error) {
		var o OrderSummary
		err := row.Scan(
			&o.ID,
			&o.Number,
			&o.Status,
			&o.CreatedAt,
			&o.TotalCents,
			&o.Count,
		)
		return o, err
	})
	if err != nil {
		return nil, fmt.Errorf("orders: customer orders: %w", err)
	}
	return out, nil
}

// adminOrders returns one page of orders for the admin and the number of matches over all
// pages. Text is an order number or part of the name, email or phone, ignoring case; strpos
// rather than LIKE, so % and _ aren't wildcards.
func (store) adminOrders(ctx context.Context, q db.DBTX, f AdminOrderFilter, staleBefore time.Time) ([]AdminOrderSummary, int, error) {
	query := `
		SELECT o.id, o.number, o.status, o.payment, o.paid_at, o.created_at, o.status_at,
		       o.status = 'supplier_pending' AND o.status_at < $5,
		       o.name, o.email, o.total_cents,
		       (SELECT coalesce(sum(l.quantity), 0) FROM orders.order_lines l WHERE l.order_id = o.id),
		       count(*) OVER ()
		FROM orders.orders o
		WHERE ($1 = '' OR o.number::text = $1
		               OR strpos(lower(o.name), lower($1)) > 0
		               OR strpos(lower(o.email), lower($1)) > 0
		               OR strpos(o.phone, $1) > 0)
		  AND ($2 = '' OR o.status = $2)
		  AND (NOT $3 OR o.status = ANY($4))
		ORDER BY
		  CASE WHEN $3 THEN o.status_at END ASC,
		  CASE WHEN $3 THEN o.number END ASC,
		  o.created_at DESC, o.number DESC
		LIMIT $6 OFFSET $7`

	args := []any{
		f.Text,
		f.Status,
		f.NeedsAction,
		needsAction,
		staleBefore,
		f.PageSize,
		(f.Page - 1) * f.PageSize,
	}

	rows, err := q.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("orders: admin orders: %w", err)
	}
	var total int
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (AdminOrderSummary, error) {
		var o AdminOrderSummary
		err := row.Scan(
			&o.ID,
			&o.Number,
			&o.Status,
			&o.Payment,
			&o.PaidAt,
			&o.CreatedAt,
			&o.StatusSince,
			&o.Stale,
			&o.Name,
			&o.Email,
			&o.TotalCents,
			&o.Count,
			&total,
		)
		return o, err
	})
	if err != nil {
		return nil, 0, fmt.Errorf("orders: admin orders: %w", err)
	}
	return out, total, nil
}

// adminLines returns an order's lines with the supplier's side, in their order.
func (store) adminLines(ctx context.Context, q db.DBTX, id uuid.UUID) ([]AdminOrderLine, error) {
	query := `
		SELECT product_id, slug, name, image, quantity, retail_cents, provider, provider_sku,
		       dealer_cents, vat_bp
		FROM orders.order_lines
		WHERE order_id = $1
		ORDER BY position`

	rows, err := q.Query(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("orders: admin lines of order %s: %w", id, err)
	}
	lines, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (AdminOrderLine, error) {
		var l AdminOrderLine
		err := row.Scan(
			&l.ProductID,
			&l.Slug,
			&l.Name,
			&l.Image,
			&l.Quantity,
			&l.UnitCents,
			&l.Provider,
			&l.ProviderSKU,
			&l.DealerCents,
			&l.VATbp,
		)
		l.TotalCents = l.UnitCents * money.Cents(l.Quantity)
		return l, err
	})
	if err != nil {
		return nil, fmt.Errorf("orders: admin lines of order %s: %w", id, err)
	}
	return lines, nil
}

// transitions returns an order's history, oldest first.
func (store) transitions(ctx context.Context, q db.DBTX, id uuid.UUID) ([]Transition, error) {
	query := `
		SELECT seq, event, from_status, to_status, at, actor, note
		FROM orders.order_transitions
		WHERE order_id = $1
		ORDER BY seq`

	rows, err := q.Query(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("orders: transitions of order %s: %w", id, err)
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Transition, error) {
		var t Transition
		err := row.Scan(
			&t.Seq,
			&t.Event,
			&t.From,
			&t.To,
			&t.At,
			&t.Actor.Email,
			&t.Actor.Note,
		)
		return t, err
	})
	if err != nil {
		return nil, fmt.Errorf("orders: transitions of order %s: %w", id, err)
	}
	return out, nil
}

// statusSince returns when an order took its status, and its seq.
func (store) statusSince(ctx context.Context, q db.DBTX, id uuid.UUID) (time.Time, int, error) {
	query := `SELECT status_at, seq FROM orders.orders WHERE id = $1`

	var since time.Time
	var seq int
	err := q.QueryRow(ctx, query, id).Scan(
		&since,
		&seq,
	)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return time.Time{}, 0, ErrNotFound
	case err != nil:
		return time.Time{}, 0, fmt.Errorf("orders: status of order %s: %w", id, err)
	}
	return since, seq, nil
}

func (store) adminCounts(ctx context.Context, q db.DBTX, staleBefore time.Time) (AdminCounts, error) {
	query := `
		SELECT count(*) FILTER (WHERE status = 'supplier_pending'),
		       count(*) FILTER (WHERE status = 'supplier_pending' AND status_at < $1),
		       count(*) FILTER (WHERE status = 'refunding')
		FROM orders.orders
		WHERE status IN ('supplier_pending', 'refunding')`

	var c AdminCounts
	err := q.QueryRow(ctx, query, staleBefore).Scan(
		&c.SupplierPending,
		&c.Stale,
		&c.Refunding,
	)
	if err != nil {
		return AdminCounts{}, fmt.Errorf("orders: admin counts: %w", err)
	}
	return c, nil
}
