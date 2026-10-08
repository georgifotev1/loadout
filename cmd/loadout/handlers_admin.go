package main

import (
	"net/http"

	"github.com/gfotev/loadout/internal/specs"
)

// GetAdminOverview handles GET /api/admin/overview.
func (app *application) GetAdminOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	queue, err := app.specs.ReviewQueue(ctx, specs.ReviewFilter{Page: 1, PageSize: 1})
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	discarded, err := app.jobs.DiscardedCount(ctx)
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	products, err := app.catalog.ProductCounts(ctx)
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	runs, err := app.catalog.SyncRuns(ctx, 1, 1)
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	orderCounts, err := app.orders.AdminCounts(ctx)
	if err != nil {
		app.handleError(w, r, err)
		return
	}

	resp := AdminOverview{
		SpecsIncomplete: queue.Total,
		DiscardedJobs:   discarded,
		Products:        ProductCounts{Active: products.Active, Inactive: products.Inactive},
		Orders: AdminOrderCounts{
			SupplierPending: orderCounts.SupplierPending,
			Stale:           orderCounts.Stale,
			Refunding:       orderCounts.Refunding,
		},
	}
	if len(runs.Runs) > 0 {
		last := syncRunJSON(runs.Runs[0])
		resp.LastSync = &last
	}
	app.writeJSON(w, r, http.StatusOK, resp)
}
