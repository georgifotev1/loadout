package specs

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gfotev/loadout/internal/platform/db"
	"github.com/gfotev/loadout/internal/platform/db/dbtest"
	"github.com/gfotev/loadout/internal/platform/validate"
)

// board is a motherboard whose supplier gave everything but the form factor.
func board(id uuid.UUID) Input {
	return Input{
		ProductID: id,
		Category:  "motherboard",
		Name:      "Дънна платка GIGABYTE B550I AORUS PRO AX",
		Values: map[string]any{
			"socket":       "AM4",
			"chipset":      "B550",
			"memory_type":  "DDR4",
			"memory_slots": 2,
			"m2_slots":     2,
		},
	}
}

func save(t *testing.T, pool *pgxpool.Pool, svc *Service, in Input) Status {
	t.Helper()
	var status Status
	err := db.InTx(context.Background(), pool, func(tx pgx.Tx) error {
		var err error
		status, err = svc.SaveTx(context.Background(), tx, in)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return status
}

func TestSaveTx(t *testing.T) {
	pool := dbtest.New(t)
	svc := NewService(pool)
	ctx := context.Background()

	t.Run("stores specs and what's missing", func(t *testing.T) {
		id := uuid.New()
		if st := save(t, pool, svc, board(id)); st != StatusIncomplete {
			t.Fatalf("status %s, want incomplete", st)
		}
		ps, err := svc.Specs(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if ps.Status != StatusIncomplete || ps.BuilderEligible || !slices.Equal(ps.Missing, []string{"form_factor"}) ||
			ps.Specs["socket"] != "AM4" || ps.Specs["memory_slots"] != 2.0 {
			t.Errorf("product specs %+v", ps)
		}
	})

	t.Run("repeating it writes nothing", func(t *testing.T) {
		id := uuid.New()
		save(t, pool, svc, board(id))
		// The product_specs row is locked (so rewritten) on every save; the field rows aren't.
		fieldVersions := func() string {
			var v string
			err := pool.QueryRow(ctx,
				`SELECT string_agg(field || ':' || xmin::text, ',' ORDER BY field) FROM specs.spec_fields WHERE product_id = $1`, id).Scan(&v)
			if err != nil {
				t.Fatal(err)
			}
			return v
		}
		before := fieldVersions()
		save(t, pool, svc, board(id))
		if after := fieldVersions(); after != before {
			t.Errorf("field rows rewritten: %s → %s", before, after)
		}
	})

	t.Run("admin value fills a gap and survives the next sync", func(t *testing.T) {
		id := uuid.New()
		save(t, pool, svc, board(id))
		if st, err := svc.SetFields(ctx, id, map[string]any{"form_factor": "ITX"}); err != nil || st != StatusApproved {
			t.Fatalf("SetFields: status %s, err %v", st, err)
		}

		if st := save(t, pool, svc, board(id)); st != StatusApproved {
			t.Fatalf("status %s, want approved", st)
		}
		ps, _ := svc.Specs(ctx, id)
		if ps.Specs["form_factor"] != "ITX" || !ps.BuilderEligible || len(ps.Missing) != 0 {
			t.Errorf("product specs %+v", ps)
		}
	})

	t.Run("a supplier value that disappears is removed, admin values stay", func(t *testing.T) {
		id := uuid.New()
		save(t, pool, svc, board(id))
		if _, err := svc.SetFields(ctx, id, map[string]any{"max_memory_gb": 64}); err != nil {
			t.Fatal(err)
		}
		in := board(id)
		delete(in.Values, "chipset")
		save(t, pool, svc, in)

		ps, _ := svc.Specs(ctx, id)
		if _, ok := ps.Specs["chipset"]; ok || ps.Specs["max_memory_gb"] != 64.0 || !slices.Contains(ps.Missing, "chipset") {
			t.Errorf("product specs %+v", ps)
		}
	})

	t.Run("an invalid value stores nothing", func(t *testing.T) {
		id := uuid.New()
		in := board(id)
		in.Values["memory_slots"] = "two"
		err := db.InTx(ctx, pool, func(tx pgx.Tx) error {
			_, err := svc.SaveTx(ctx, tx, in)
			return err
		})
		if !errors.Is(err, ErrInvalidValue) {
			t.Fatalf("err = %v, want ErrInvalidValue", err)
		}
		if _, err := svc.Specs(ctx, id); !errors.Is(err, ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("moving to a category without specs deletes them", func(t *testing.T) {
		id := uuid.New()
		save(t, pool, svc, board(id))
		in := board(id)
		in.Category, in.Values = "peripheral.mouse", nil
		if st := save(t, pool, svc, in); st != "" {
			t.Errorf("status %q, want none", st)
		}
		if _, err := svc.Specs(ctx, id); !errors.Is(err, ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("concurrent saves of one product serialize", func(t *testing.T) {
		id := uuid.New()
		var wg sync.WaitGroup
		errs := make(chan error, 10)
		for range 10 {
			wg.Go(func() {
				errs <- db.InTx(ctx, pool, func(tx pgx.Tx) error {
					_, err := svc.SaveTx(ctx, tx, board(id))
					return err
				})
			})
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Error(err)
			}
		}
	})
}

func TestSetFields(t *testing.T) {
	pool := dbtest.New(t)
	svc := NewService(pool)
	ctx := context.Background()

	t.Run("admin value beats the supplier's", func(t *testing.T) {
		id := uuid.New()
		save(t, pool, svc, board(id))
		if _, err := svc.SetFields(ctx, id, map[string]any{"memory_slots": 4, "form_factor": "ATX"}); err != nil {
			t.Fatal(err)
		}
		ps, _ := svc.Specs(ctx, id)
		if ps.Specs["memory_slots"] != 4.0 || ps.Specs["socket"] != "AM4" || ps.Status != StatusApproved {
			t.Errorf("product specs %+v", ps)
		}
	})

	t.Run("product without specs", func(t *testing.T) {
		if _, err := svc.SetFields(ctx, uuid.New(), map[string]any{"socket": "AM4"}); !errors.Is(err, ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("an invalid value stores nothing", func(t *testing.T) {
		id := uuid.New()
		save(t, pool, svc, board(id))
		_, err := svc.SetFields(ctx, id, map[string]any{"form_factor": "ITX", "memory_slots": "two"})
		var verr *validate.Error
		if !errors.As(err, &verr) || len(verr.Fields) != 1 || verr.Fields[0] != (validate.FieldError{Field: "memory_slots", Code: "invalid_type"}) {
			t.Fatalf("err = %v, want memory_slots: invalid_type", err)
		}
		if ps, _ := svc.Specs(ctx, id); ps.Status != StatusIncomplete {
			t.Errorf("status %s, want incomplete", ps.Status)
		}
	})

	t.Run("nil clears the admin value and the supplier's counts again", func(t *testing.T) {
		id := uuid.New()
		save(t, pool, svc, board(id))
		if _, err := svc.SetFields(ctx, id, map[string]any{"socket": "AM5", "form_factor": "ITX"}); err != nil {
			t.Fatal(err)
		}
		status, err := svc.SetFields(ctx, id, map[string]any{"socket": nil, "form_factor": nil})
		if err != nil {
			t.Fatal(err)
		}
		ps, _ := svc.Specs(ctx, id)
		if status != StatusIncomplete || ps.Specs["socket"] != "AM4" || !slices.Equal(ps.Missing, []string{"form_factor"}) {
			t.Errorf("status %s, specs %+v", status, ps)
		}
		// The next sync doesn't bring the cleared values back.
		save(t, pool, svc, board(id))
		if ps, _ := svc.Specs(ctx, id); ps.Specs["socket"] != "AM4" {
			t.Errorf("after sync %+v", ps.Specs)
		}
	})
}

func TestFields(t *testing.T) {
	pool := dbtest.New(t)
	svc := NewService(pool)
	ctx := context.Background()

	id := uuid.New()
	in := board(id)
	in.Values["max_memory_gb"] = nil // the adapter doesn't know it
	save(t, pool, svc, in)
	if _, err := svc.SetFields(ctx, id, map[string]any{"memory_slots": 4}); err != nil {
		t.Fatal(err)
	}

	pf, err := svc.Fields(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if pf.ProductID != id || pf.Status != StatusIncomplete || len(pf.Fields) != len(schemas["motherboard"].fields) {
		t.Fatalf("product fields %+v", pf)
	}
	for _, f := range pf.Fields {
		switch f.Name {
		case "memory_slots":
			if f.Supplier != 2.0 || f.Admin != 4.0 || f.Value != 4.0 || !f.Required {
				t.Errorf("memory_slots %+v", f)
			}
		case "max_memory_gb":
			if f.Supplier != nil || f.Value != nil || f.Required {
				t.Errorf("max_memory_gb %+v", f)
			}
		}
	}

	if _, err := svc.Fields(ctx, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown product: err = %v, want ErrNotFound", err)
	}
}

func TestEligiblePartsAndSpecsByIDs(t *testing.T) {
	pool := dbtest.New(t)
	svc := NewService(pool)
	ctx := context.Background()

	incomplete, complete := uuid.New(), uuid.New()
	save(t, pool, svc, board(incomplete))
	in := board(complete)
	in.Values["form_factor"] = "ITX"
	save(t, pool, svc, in)

	parts, err := svc.EligibleParts(ctx, "motherboard")
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 1 || parts[0].ProductID != complete {
		t.Errorf("eligible %+v, want only the complete board", parts)
	}
	if parts, _ := svc.EligibleParts(ctx, "peripheral.mouse"); len(parts) != 0 {
		t.Errorf("peripherals: %+v", parts)
	}

	byID, err := svc.SpecsByIDs(ctx, []uuid.UUID{incomplete, complete, uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	if len(byID) != 2 || byID[incomplete].Status != StatusIncomplete || byID[complete].Specs["form_factor"] != "ITX" {
		t.Errorf("by id %+v", byID)
	}
}

func TestReviewQueue(t *testing.T) {
	pool := dbtest.New(t)
	svc := NewService(pool)
	ctx := context.Background()

	first, second := uuid.New(), uuid.New()
	save(t, pool, svc, board(first))
	in := board(second)
	in.Category, in.Values = "ram", map[string]any{"type": "DDR4"}
	save(t, pool, svc, in)
	complete := board(uuid.New())
	complete.Values["form_factor"] = "ITX"
	if st := save(t, pool, svc, complete); st != StatusApproved {
		t.Fatalf("status %s, want approved", st)
	}

	t.Run("incomplete only, oldest first", func(t *testing.T) {
		page, err := svc.ReviewQueue(ctx, ReviewFilter{Page: 1, PageSize: 10})
		if err != nil {
			t.Fatal(err)
		}
		if page.Total != 2 || len(page.Items) != 2 || page.Items[0].ProductID != first || page.Items[1].ProductID != second {
			t.Fatalf("page %+v, want the board then the RAM", page)
		}
		if !slices.Equal(page.Items[1].Missing, []string{"capacity_gb_total", "modules", "speed_mhz"}) {
			t.Errorf("RAM missing %v", page.Items[1].Missing)
		}
	})

	t.Run("by category", func(t *testing.T) {
		page, err := svc.ReviewQueue(ctx, ReviewFilter{Category: "ram", Page: 1, PageSize: 10})
		if err != nil {
			t.Fatal(err)
		}
		if page.Total != 1 || page.Items[0].ProductID != second {
			t.Errorf("page %+v", page)
		}
	})

	t.Run("invalid filter", func(t *testing.T) {
		_, err := svc.ReviewQueue(ctx, ReviewFilter{Category: "toaster", Page: 0, PageSize: 1000})
		var verr *validate.Error
		if !errors.As(err, &verr) || len(verr.Fields) != 3 {
			t.Errorf("err %v, want 3 field errors", err)
		}
	})
}
